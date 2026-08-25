package process

import (
	"cmp"
	"context"
	"io"
	"strings"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/utils"
)

func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	record, _, err := e.workloadMeta(ctx, ID)
	if err != nil {
		return "", nil, nil, nil, err
	}
	argv := scopeArgv(record, config)
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{Stdin: config.AttachStdin, TTY: config.Tty})
	if err != nil {
		return "", nil, nil, nil, err
	}

	execID = utils.RandomID()
	e.execs.Add(execID, running)
	if config.AttachStdin {
		return execID, running.Stdout(), nil, running.Stdin(), nil
	}
	return execID, running.Stdout(), running.Stderr(), nil, nil
}

func (e *Engine) ExecResize(_ context.Context, execID string, height, width uint) error {
	return e.execs.Resize(execID, height, width)
}

func (e *Engine) ExecExitCode(_ context.Context, _, execID string) (int, error) {
	return e.execs.ExitCode(execID)
}

// scopeArgv runs the command in the workload's own slice and root, without namespaces.
func scopeArgv(record *meta, config *enginetypes.ExecConfig) []string {
	argv := []string{"systemd-run", "--scope", "--quiet", "--collect", "--slice=" + sliceName(record.Podname)}
	for _, env := range config.Env {
		argv = append(argv, "--setenv="+env)
	}
	argv = append(argv, "--")

	switch {
	case record.RootDirectory != "":
		argv = append(argv, "chroot")
		if config.User != "" {
			argv = append(argv, "--userspec="+config.User)
		}
		argv = append(argv, record.RootDirectory)
	case config.User != "":
		user, group, _ := strings.Cut(config.User, ":")
		argv = append(argv, "setpriv", "--reuid="+user, "--regid="+cmp.Or(group, user), "--init-groups", "--")
	}
	if config.WorkingDir != "" && config.WorkingDir != "/" {
		argv = append(argv, "env", "--chdir="+config.WorkingDir)
	}
	return append(argv, config.Cmd...)
}
