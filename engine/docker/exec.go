package docker

import (
	"context"
	"io"
	"io/ioutil"

	dockertypes "github.com/docker/docker/api/types"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// ExecCreate create a exec
func (e *Engine) ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, error) {
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
func (e *Engine) ExecAttach(ctx context.Context, execID string, tty bool) (io.ReadCloser, io.WriteCloser, error) {
	execStartCheck := dockertypes.ExecStartCheck{
		Tty: tty,
	}
	resp, err := e.client.ContainerExecAttach(ctx, execID, execStartCheck)
	if err != nil {
		return nil, nil, err
	}
	return ioutil.NopCloser(resp.Reader), resp.Conn, nil
}

// ExecExitCode get exec return code
func (e *Engine) ExecExitCode(ctx context.Context, execID string) (int, error) {
	r, err := e.client.ContainerExecInspect(ctx, execID)
	if err != nil {
		return -1, err
	}
	return r.ExitCode, nil
}

// ExecResize resize exec tty
func (e *Engine) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	opts := dockertypes.ResizeOptions{
		Height: height,
		Width:  width,
	}

	return e.client.ContainerExecResize(ctx, execID, opts)
}
