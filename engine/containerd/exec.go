package containerd

import (
	"context"
	"io"
	"slices"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// ctrBinary ships with containerd; a task's stdio lives in node-local fifos, so an exec
// core can stream has to start on the node.
const ctrBinary = "ctr"

var errExecNotFound = errors.New("exec not found")

func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	found, err := e.container(ctx, ID)
	if err != nil {
		return "", nil, nil, nil, err
	}
	execID = newID()
	argv := e.execArgv(found.ID(), execID, config)
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{Stdin: config.AttachStdin, TTY: config.Tty})
	if err != nil {
		return "", nil, nil, nil, err
	}

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

// execArgv renders one `ctr tasks exec`; ctr has no env flag, so env rides in the command.
func (e *Engine) execArgv(ID, execID string, config *enginetypes.ExecConfig) []string {
	argv := []string{ctrBinary, "--address", e.socket, "--namespace", e.namespace, "tasks", "exec", "--exec-id", execID}
	if config.Tty {
		argv = append(argv, "--tty")
	}
	if config.User != "" {
		argv = append(argv, "--user", config.User)
	}
	if config.WorkingDir != "" {
		argv = append(argv, "--cwd", config.WorkingDir)
	}
	argv = append(argv, ID)
	if len(config.Env) > 0 {
		argv = slices.Concat(argv, []string{"env"}, config.Env)
	}
	return append(argv, config.Cmd...)
}
