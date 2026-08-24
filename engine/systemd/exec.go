package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

func (e *Engine) Execute(_ context.Context, _ string, _ *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return execID, stdout, stderr, writer, err
}

func (e *Engine) ExecResize(_ context.Context, _ string, _, _ uint) (err error) {
	err = types.ErrEngineNotImplemented
	return err
}

func (e *Engine) ExecExitCode(_ context.Context, _, _ string) (execCode int, err error) {
	err = types.ErrEngineNotImplemented
	return execCode, err
}
