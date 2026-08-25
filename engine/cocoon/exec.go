package cocoon

import (
	"cmp"
	"context"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

var errExecNotFound = errors.New("exec not found")

// Execute runs the command through cocoon-agent in pipe mode; a pty is projecteru2/core#660.
func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	argv, err := e.guestArgv(ctx, ID, config)
	if err != nil {
		return "", nil, nil, nil, err
	}
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{Stdin: config.AttachStdin})
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

func (e *Engine) guestArgv(ctx context.Context, ID string, config *enginetypes.ExecConfig) ([]string, error) {
	if config.User == "" && config.WorkingDir == "" {
		return e.execArgv(ID, config, config.Cmd), nil
	}
	_, vm, err := e.inspectVM(ctx, ID)
	if err != nil {
		return nil, err
	}
	if vm.Config.Windows {
		return nil, errors.Wrap(coretypes.ErrEngineNotImplemented,
			"a windows guest has neither runuser nor setpriv to run an exec as another user or in another directory (projecteru2/core#660)")
	}
	return e.execArgv(ID, config, guestCommand(config)), nil
}

func (e *Engine) execArgv(ID string, config *enginetypes.ExecConfig, cmd []string) []string {
	argv := e.vm("exec")
	if config.AttachStdin {
		argv = append(argv, "-i")
	}
	for _, env := range config.Env {
		argv = append(argv, "-e", env)
	}
	return slices.Concat(argv, []string{ID, "--"}, cmd)
}

func guestCommand(config *enginetypes.ExecConfig) []string {
	var argv []string
	if config.User != "" {
		argv = userArgv(config.User)
	}
	if config.WorkingDir != "" {
		argv = append(argv, "env", "--chdir="+config.WorkingDir)
	}
	return append(argv, config.Cmd...)
}

func userArgv(user string) []string {
	name, group, _ := strings.Cut(user, ":")
	if _, err := strconv.Atoi(name); err == nil || group != "" {
		return []string{"setpriv", "--reuid=" + name, "--regid=" + cmp.Or(group, name), "--clear-groups", "--"}
	}
	return []string{"runuser", "-u", name, "--"}
}
