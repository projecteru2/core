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
	coretypes "github.com/projecteru2/core/types"
)

// logFlushGrace lets journald hand over the last lines a dying task wrote.
const logFlushGrace = time.Second

// sessionReader closes the ssh session backing a follow stream.
type sessionReader struct {
	io.Reader
	sess sshrunner.Session
}

func (r *sessionReader) Close() error {
	return r.sess.Close()
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
	return &sessionReader{Reader: running.Stdout(), sess: running}, nil, nil
}

func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if stdin {
		return nil, nil, nil, coretypes.ErrEngineNotImplemented
	}
	stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
	return stdout, stderr, nil, err
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
