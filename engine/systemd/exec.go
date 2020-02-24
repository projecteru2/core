package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
)

func (s *SystemdSSH) ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (execID string, err error) {
	return
}

func (s *SystemdSSH) ExecAttach(ctx context.Context, execID string, tty bool) (reader io.ReadCloser, writer io.WriteCloser, err error) {
	return
}

func (s *SystemdSSH) Execute(ctx context.Context, target string, config *enginetypes.ExecConfig) (execID string, reader io.ReadCloser, writer io.WriteCloser, err error) {
	return
}

func (s *SystemdSSH) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	return
}

func (s *SystemdSSH) ExecExitCode(ctx context.Context, execID string) (execCode int, err error) {
	return
}
