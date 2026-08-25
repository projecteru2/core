package process

import (
	"context"
	"io"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
)

var errExecNotFound = errors.New("exec not found")

func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	record, err := e.workloadMeta(ctx, ID)
	if err != nil {
		return "", nil, nil, nil, err
	}
	argv := scopeArgv(record, config)
	running, err := e.runner.Start(ctx, quote(argv), &startOptions{Stdin: config.AttachStdin, TTY: config.Tty})
	if err != nil {
		return "", nil, nil, nil, err
	}

	execID = newID()
	e.mu.Lock()
	e.execs[execID] = running
	e.mu.Unlock()
	if config.AttachStdin {
		return execID, running.Stdout(), nil, running.Stdin(), nil
	}
	return execID, running.Stdout(), running.Stderr(), nil, nil
}

func (e *Engine) ExecResize(_ context.Context, execID string, height, width uint) error {
	e.mu.Lock()
	running, ok := e.execs[execID]
	e.mu.Unlock()
	if !ok {
		return errors.Wrap(errExecNotFound, execID)
	}
	return running.Resize(height, width)
}

func (e *Engine) ExecExitCode(_ context.Context, _, execID string) (int, error) {
	e.mu.Lock()
	running, ok := e.execs[execID]
	delete(e.execs, execID)
	e.mu.Unlock()
	if !ok {
		return -1, errors.Wrap(errExecNotFound, execID)
	}
	defer func() {
		_ = running.Close()
	}()
	return running.Wait()
}

// scopeArgv runs the command in the workload's own slice and root, without namespaces.
func scopeArgv(record *meta, config *enginetypes.ExecConfig) []string {
	argv := []string{"systemd-run", "--scope", "--quiet", "--collect", "--slice=" + sliceName(record.Podname)}
	if config.User != "" {
		argv = append(argv, "--uid="+config.User)
	}
	if record.RootDirectory != "" {
		argv = append(argv, "-p", "RootDirectory="+record.RootDirectory)
	}
	if config.WorkingDir != "" {
		argv = append(argv, "--working-directory="+config.WorkingDir)
	}
	for _, env := range config.Env {
		argv = append(argv, "--setenv="+env)
	}
	return append(append(argv, "--"), config.Cmd...)
}
