package containerd

import (
	"context"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/containerd/containerd/v2/client"

	"github.com/projecteru2/core/engine/journal"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// logFlushGrace lets journald hand over the last lines a dying task wrote.
const logFlushGrace = time.Second

// attach is the `ctr tasks start` session an interactive workload runs under: it owns the
// task's fifos, so it is at once the workload's stdio, its console and its exit status.
type attach struct {
	sess   sshrunner.Session
	exited <-chan client.ExitStatus
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

// VirtualizationAttach without stdin is the journald follow; with stdin it hands back the
// session the workload was started under, which is the only thing holding its fifos.
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if !stdin {
		stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
		return stdout, stderr, nil, err
	}

	e.mu.Lock()
	running, ok := e.attaches[ID]
	e.mu.Unlock()
	if !ok {
		return nil, nil, nil, errors.Wrap(errAttachNotFound, ID)
	}
	return io.NopCloser(running.sess.Stdout()), io.NopCloser(running.sess.Stderr()), running.sess.Stdin(), nil
}

// startInteractive runs the task under `ctr tasks start`, which reads process.terminal off the
// spec, makes the fifos on the node and relays them to the session's own stdio.
func (e *Engine) startInteractive(ctx context.Context, ID string) error {
	argv := []string{ctrBinary, "--address", e.socket, "--namespace", e.namespace, "tasks", "start", ID}
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{Stdin: true, TTY: true})
	if err != nil {
		return err
	}

	exited := make(chan client.ExitStatus, 1)
	go waitSession(running, exited)
	e.mu.Lock()
	e.attaches[ID] = &attach{sess: running, exited: exited}
	e.mu.Unlock()
	return nil
}

// waitSession turns ctr's own exit into the workload's: it exits with the task's status.
func waitSession(running sshrunner.Session, exited chan<- client.ExitStatus) {
	code, err := running.Wait()
	_ = running.Close()
	exited <- *client.NewExitStatus(uint32(max(code, 0)), time.Now(), err) //nolint:gosec // a shell exit status is never past 255
	close(exited)
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
