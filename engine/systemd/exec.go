package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// Execute executes a cmd and attaches stdio
func (s *systemdEngine) Execute(ctx context.Context, target string, config *enginetypes.ExecConfig) (execID string, stdout io.ReadCloser, stderr io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecResize resize the terminal size
func (s *systemdEngine) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecExitCode fetches exceuction exit code
func (s *systemdEngine) ExecExitCode(ctx context.Context, execID string) (execCode int, err error) {
	err = types.ErrEngineNotImplemented
	return
}
