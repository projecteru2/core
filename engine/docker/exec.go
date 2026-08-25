package docker

import (
	"context"
	"io"

	"github.com/moby/moby/api/pkg/stdcopy"
	dockerapi "github.com/moby/moby/client"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
)

func (e *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	if execID, err = e.execCreate(ctx, ID, config); err != nil {
		return execID, stdout, stderr, stdin, err
	}

	reader, writer, err := e.execAttach(ctx, execID, config.Tty)
	if err != nil {
		return execID, stdout, stderr, stdin, err
	}
	if config.AttachStdin {
		return execID, reader, nil, writer, err
	}

	stdout, stderr = e.demultiplexStdStream(ctx, reader)
	return execID, stdout, stderr, nil, err
}

func (e *Engine) ExecResize(ctx context.Context, execID string, height, width uint) error {
	opts := dockerapi.ExecResizeOptions{
		Height: height,
		Width:  width,
	}

	_, err := e.client.ExecResize(ctx, execID, opts)
	return err
}

func (e *Engine) ExecExitCode(ctx context.Context, _, execID string) (int, error) {
	r, err := e.client.ExecInspect(ctx, execID, dockerapi.ExecInspectOptions{})
	if err != nil {
		return -1, err
	}
	return r.ExitCode, nil
}

func (e *Engine) execCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, error) {
	execConfig := dockerapi.ExecCreateOptions{
		User:         config.User,
		Privileged:   config.Privileged,
		Cmd:          config.Cmd,
		WorkingDir:   config.WorkingDir,
		Env:          config.Env,
		AttachStderr: config.AttachStderr,
		AttachStdout: config.AttachStdout,
		AttachStdin:  config.AttachStdin,
		TTY:          config.Tty,
	}

	idResp, err := e.client.ExecCreate(ctx, target, execConfig)
	if err != nil {
		return "", err
	}
	return idResp.ID, nil
}

func (e *Engine) execAttach(ctx context.Context, execID string, tty bool) (io.ReadCloser, io.WriteCloser, error) {
	resp, err := e.client.ExecAttach(ctx, execID, dockerapi.ExecAttachOptions{TTY: tty})
	if err != nil {
		return nil, nil, err
	}
	return io.NopCloser(resp.Reader), resp.Conn, nil
}

func (e *Engine) demultiplexStdStream(ctx context.Context, stdStream io.Reader) (stdout, stderr io.ReadCloser) {
	stdout, stdoutW := io.Pipe()
	stderr, stderrW := io.Pipe()
	go func() {
		defer func() {
			_ = stdoutW.Close()
			_ = stderrW.Close()
		}()
		if _, err := stdcopy.StdCopy(stdoutW, stderrW, stdStream); err != nil {
			log.WithFunc("engine.docker.demultiplexStdStream").Error(ctx, err, "stdcopy failed")
		}
	}()
	return stdout, stderr
}
