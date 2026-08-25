package cocoon

import (
	"context"
	"io"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

var errExecNotFound = errors.New("exec not found")

// Execute runs the command through cocoon-agent in pipe mode; a pty is projecteru2/core#660.
func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	running, err := e.runner.Start(ctx, sshrunner.Quote(e.execArgv(ID, config)), &sshrunner.StartOptions{Stdin: config.AttachStdin})
	if err != nil {
		return "", nil, nil, nil, err
	}

	execID = newID()
	e.mu.Lock()
	e.execs[execID] = running
	e.mu.Unlock()
	if config.AttachStdin {
		stdin = running.Stdin()
	}
	return execID, running.Stdout(), running.Stderr(), stdin, nil
}

func (e *Engine) ExecResize(context.Context, string, uint, uint) error {
	return coretypes.ErrEngineNotImplemented
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

func (e *Engine) execArgv(ID string, config *enginetypes.ExecConfig) []string {
	argv := e.vm("exec")
	if config.AttachStdin {
		argv = append(argv, "-i")
	}
	for _, env := range config.Env {
		argv = append(argv, "-e", env)
	}
	return append(append(argv, ID, "--"), config.Cmd...)
}
