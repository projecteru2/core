package docker

import (
	"context"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/pkg/stdcopy"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
)

// ExecCreate create a exec
func (e *Engine) execCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, error) {
	execConfig := dockertypes.ExecConfig{
		User:         config.User,
		Privileged:   config.Privileged,
		Cmd:          config.Cmd,
		WorkingDir:   config.WorkingDir,
		Env:          config.Env,
		AttachStderr: config.AttachStderr,
		AttachStdout: config.AttachStdout,
		AttachStdin:  config.AttachStdin,
		Tty:          config.Tty,
	}

	// TODO should timeout
	// Fuck docker, ctx will not use inside funcs!!
	idResp, err := e.client.ContainerExecCreate(ctx, target, execConfig)
	if err != nil {
		return "", err
	}
	return idResp.ID, nil
}

// ExecAttach attach a exec
func (e *Engine) execAttach(ctx context.Context, execID string, tty bool) (io.ReadCloser, io.WriteCloser, error) {
	execStartCheck := dockertypes.ExecStartCheck{
		Tty: tty,
	}
	resp, err := e.client.ContainerExecAttach(ctx, execID, execStartCheck)
	if err != nil {
		return nil, nil, err
	}
	return io.NopCloser(resp.Reader), resp.Conn, nil
}

// Execute executes a workload
func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	if execID, err = e.execCreate(ctx, ID, config); err != nil {
		return
	}

	reader, writer, err := e.execAttach(ctx, execID, config.Tty)
	if err != nil {
		return
	}
	if config.AttachStdin {
		return execID, reader, nil, writer, err
	}

	stdout, stderr = e.demultiplexStdStream(ctx, reader)
	return execID, stdout, stderr, nil, err
}

func (e *Engine) demultiplexStdStream(ctx context.Context, stdStream io.Reader) (stdout, stderr io.ReadCloser) {
	stdout, stdoutW := io.Pipe()
	stderr, stderrW := io.Pipe()
	go func() {
		defer stdoutW.Close()
		defer stderrW.Close()
		if _, err := stdcopy.StdCopy(stdoutW, stderrW, stdStream); err != nil {
			log.Error(ctx, err, "[docker.demultiplex] StdCopy failed")
		}
	}()
	return stdout, stderr
}

// ExecExitCode get exec return code
func (e *Engine) ExecExitCode(ctx context.Context, ID, execID string) (int, error) {
	r, err := e.client.ContainerExecInspect(ctx, execID)
	if err != nil {
		return -1, err
	}
	return r.ExitCode, nil
}

// ExecResize resize exec tty
func (e *Engine) ExecResize(ctx context.Context, execID string, height, width uint) error {
	opts := dockertypes.ResizeOptions{
		Height: height,
		Width:  width,
	}

	return e.client.ContainerExecResize(ctx, execID, opts)
}
