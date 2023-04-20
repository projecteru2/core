package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// Execute executes a cmd and attaches stdio
func (e *Engine) Execute(_ context.Context, _ string, _ *enginetypes.ExecConfig) (execID string, stdout io.ReadCloser, stderr io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecResize resize the terminal size
func (e *Engine) ExecResize(_ context.Context, _ string, _, _ uint) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ExecExitCode fetches exceuction exit code
func (e *Engine) ExecExitCode(_ context.Context, _, _ string) (execCode int, err error) {
	err = types.ErrEngineNotImplemented
	return
}
