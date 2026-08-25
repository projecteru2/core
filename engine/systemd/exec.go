package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

func (e *Engine) Execute(context.Context, string, *enginetypes.ExecConfig) (string, io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	return "", nil, nil, nil, types.ErrEngineNotImplemented
}

func (e *Engine) ExecResize(context.Context, string, uint, uint) error {
	return types.ErrEngineNotImplemented
}

func (e *Engine) ExecExitCode(context.Context, string, string) (int, error) {
	return 0, types.ErrEngineNotImplemented
}
