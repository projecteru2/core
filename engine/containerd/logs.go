package containerd

import (
	"context"
	"io"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/cio"

	"github.com/projecteru2/core/engine/journal"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
)

const (
	// logFlushGrace lets journald hand over the last lines a dying task wrote.
	logFlushGrace = time.Second

	fifoDirName  = "fifo"
	relayStreams = 3
	stdinStream  = "stdin"

	fifoMakeScript = `set -e
mkdir -p "$1"
shift
rm -f "$@"
mkfifo "$@"
`
	fifoWriteScript = `exec cat > "$1"`
)

type attach struct {
	stdin  sshrunner.Session
	stdout sshrunner.Session
	stderr sshrunner.Session

	released   atomic.Bool
	died       chan error
	closeStdin func(context.Context) error
}

func (a *attach) close() {
	a.released.Store(true)
	for _, sess := range []sshrunner.Session{a.stdin, a.stdout, a.stderr} {
		if sess != nil {
			_ = sess.Close()
		}
	}
}

// watch reports a relay that ends on its own; its stderr is the only account of why it did.
func (a *attach) watch(ctx context.Context, ID, stream string, sess sshrunner.Session) {
	logger := log.WithFunc("engine.containerd.attach.watch").WithField("ID", ID)
	output, _ := io.ReadAll(sess.Stderr())
	code, waitErr := sess.Wait()
	reason := strings.TrimSpace(string(output))
	if a.released.Load() {
		return
	}
	if code == 0 && waitErr == nil && reason == "" {
		if stream == stdinStream {
			logger.Error(ctx, a.closeStdin(ctx))
		}
		return
	}
	err := errors.Newf("%s relay ended with code %d: %s", stream, code, reason)
	logger.Error(ctx, errors.Join(err, waitErr))
	select {
	case a.died <- err:
	default:
	}
}

func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	found, err := e.container(ctx, opts.ID)
	if err != nil {
		return nil, nil, err
	}
	flags, err := journal.Flags(opts)
	if err != nil {
		return nil, nil, err
	}
	argv := slices.Concat([]string{"journalctl", "SYSLOG_IDENTIFIER=" + journal.Identifier, "ERU_ID=" + found.ID()}, flags)

	task, taskErr := found.Task(ctx, nil)
	if !opts.Follow || taskErr != nil {
		res, runErr := e.run(ctx, argv...)
		if runErr != nil {
			return nil, nil, runErr
		}
		return io.NopCloser(strings.NewReader(res.Stdout)), nil, nil
	}

	running, err := e.runner.Start(ctx, sshrunner.Quote(append(argv, "-f")), &sshrunner.StartOptions{})
	if err != nil {
		return nil, nil, err
	}
	// journalctl -f never ends by itself, and RunAndWait drains the stream before it reads the exit code
	go endWithTask(ctx, task, running)
	return sshrunner.Reader(running), nil, nil
}

// VirtualizationAttach without stdin is the journald follow; with stdin it is the fifo relays.
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if !stdin {
		stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
		return stdout, stderr, nil, err
	}

	e.mu.Lock()
	relay, ok := e.attaches[ID]
	e.mu.Unlock()
	if !ok {
		return nil, nil, nil, errors.Wrap(errAttachNotFound, ID)
	}
	return relay.stdout.Stdout(), relay.stderr.Stdout(), relay.stdin.Stdin(), nil
}

// relayFifos parks a session on each fifo before the task exists; the shim's own open blocks on them.
func (e *Engine) relayFifos(ctx context.Context, ID string) (_ cio.Creator, err error) {
	e.releaseAttach(ID)
	set := fifoSet(ID)
	if _, err = e.run(ctx, sshrunner.Shell(fifoMakeScript, filepath.Dir(set.Stdin), set.Stdin, set.Stdout, set.Stderr)...); err != nil {
		return nil, err
	}

	// a relay lives as long as the workload, and the deploy request that starts it does not
	held := context.WithoutCancel(ctx)
	// the shim holds the stdin fifo open read-write, so only CloseIO ends the workload's input
	relay := &attach{
		died:       make(chan error, relayStreams),
		closeStdin: func(ctx context.Context) error { return e.closeTaskStdin(ctx, ID) },
	}
	defer func() {
		if err != nil {
			relay.close()
		}
	}()
	if relay.stdin, err = e.runner.Start(held, sshrunner.Quote(sshrunner.Shell(fifoWriteScript, set.Stdin)), &sshrunner.StartOptions{Stdin: true}); err != nil {
		return nil, err
	}
	if relay.stdout, err = e.runner.Start(held, sshrunner.Quote([]string{"cat", set.Stdout}), &sshrunner.StartOptions{}); err != nil {
		return nil, err
	}
	if relay.stderr, err = e.runner.Start(held, sshrunner.Quote([]string{"cat", set.Stderr}), &sshrunner.StartOptions{}); err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.attaches[ID] = relay
	e.mu.Unlock()
	go relay.watch(held, ID, stdinStream, relay.stdin)
	go relay.watch(held, ID, "stdout", relay.stdout)
	go relay.watch(held, ID, "stderr", relay.stderr)
	return func(string) (cio.IO, error) { return cio.Load(cio.NewFIFOSet(set, nil)) }, nil
}

func (e *Engine) closeTaskStdin(ctx context.Context, ID string) error {
	task, err := e.task(ctx, ID)
	if err != nil {
		return err
	}
	return task.CloseIO(ctx, client.WithStdinCloser)
}

func (e *Engine) relayFailure(ID string) error {
	e.mu.Lock()
	relay, ok := e.attaches[ID]
	e.mu.Unlock()
	if !ok {
		return nil
	}
	select {
	case err := <-relay.died:
		return err
	default:
		return nil
	}
}

func (e *Engine) releaseAttach(ID string) {
	e.mu.Lock()
	relay, ok := e.attaches[ID]
	delete(e.attaches, ID)
	e.mu.Unlock()
	if ok {
		relay.close()
	}
}

func fifoSet(ID string) cio.Config {
	dir := filepath.Join(workloadDir(ID), fifoDirName)
	return cio.Config{
		Stdin:  filepath.Join(dir, "stdin"),
		Stdout: filepath.Join(dir, "stdout"),
		Stderr: filepath.Join(dir, "stderr"),
	}
}

func endWithTask(ctx context.Context, task client.Task, running sshrunner.Session) {
	if exited, err := task.Wait(ctx); err == nil {
		select {
		case <-exited:
		case <-ctx.Done():
		}
	}
	timer := time.NewTimer(logFlushGrace)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
	}
	_ = running.Close()
}
