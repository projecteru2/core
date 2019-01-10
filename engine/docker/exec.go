package docker

import (
	"context"
	"io"

	dockertypes "github.com/docker/docker/api/types"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// ExecCreate create a exec
func (e *Engine) ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, error) {
	execConfig := dockertypes.ExecConfig{
		User:         config.User,
		Cmd:          config.Cmd,
		Privileged:   config.Privileged,
		Env:          config.Env,
		AttachStderr: config.AttachStderr,
		AttachStdout: config.AttachStdout,
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
func (e *Engine) ExecAttach(ctx context.Context, execID string, detach, tty bool) (io.ReadCloser, error) {
	resp, err := e.client.ContainerExecAttach(ctx, execID, dockertypes.ExecStartCheck{})
	if err != nil {
		return nil, err
	}
	return FuckDockerStream(resp), nil
}

// ExecExitCode get exec return code
func (e *Engine) ExecExitCode(ctx context.Context, execID string) (int, error) {
	r, err := e.client.ContainerExecInspect(ctx, execID)
	if err != nil {
		return -1, err
	}
	return r.ExitCode, nil
}
