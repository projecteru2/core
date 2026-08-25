package containerd

import (
	"context"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/containerd/containerd/v2/client"

	"github.com/projecteru2/core/engine/journal"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// logFlushGrace lets journald hand over the last lines a dying task wrote.
const logFlushGrace = time.Second

// attach is a live `ctr tasks attach`: the session core resizes, and the task exit registered
// before ctr started, because ctr deletes the task when the attach ends.
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

// VirtualizationAttach without stdin is the journald follow; with stdin it is `ctr tasks attach`
// on the node, since a task's stdio lives in fifos core cannot reach.
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if !stdin {
		stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
		return stdout, stderr, nil, err
	}

	found, err := e.container(ctx, ID)
	if err != nil {
		return nil, nil, nil, err
	}
	info, err := found.Info(ctx, client.WithoutRefreshedMetadata)
	if err != nil {
		return nil, nil, nil, err
	}
	spec, err := containerSpec(info)
	if err != nil {
		return nil, nil, nil, err
	}
	task, err := found.Task(ctx, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	exited, err := task.Wait(ctx)
	if err != nil {
		return nil, nil, nil, err
	}
	return e.startAttach(ctx, found.ID(), spec.Process != nil && spec.Process.Terminal, exited)
}

// startAttach wires the node-side attach onto the streams core reads and writes.
func (e *Engine) startAttach(ctx context.Context, ID string, tty bool, exited <-chan client.ExitStatus) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	argv := []string{ctrBinary, "--address", e.socket, "--namespace", e.namespace, "tasks", "attach", ID}
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{Stdin: true, TTY: tty})
	if err != nil {
		return nil, nil, nil, err
	}

	e.mu.Lock()
	e.attaches[ID] = &attach{sess: running, exited: exited}
	e.mu.Unlock()
	return sshrunner.Reader(running), running.Stderr(), running.Stdin(), nil
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
