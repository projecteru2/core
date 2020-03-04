package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// ExecCreate executes a cmd
func (s *SSHClient) ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (execID string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecAttach attaches stdio
func (s *SSHClient) ExecAttach(ctx context.Context, execID string, tty bool) (reader io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// Execute executes a cmd and attaches stdio
func (s *SSHClient) Execute(ctx context.Context, target string, config *enginetypes.ExecConfig) (execID string, reader io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecResize resize the terminal size
func (s *SSHClient) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecExitCode fetches exceuction exit code
func (s *SSHClient) ExecExitCode(ctx context.Context, execID string) (execCode int, err error) {
	err = types.ErrEngineNotImplemented
	return
}
