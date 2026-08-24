package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

func (e *Engine) VirtualizationLogs(context.Context, *enginetypes.VirtualizationLogStreamOptions) (io.ReadCloser, io.ReadCloser, error) {
	return nil, nil, types.ErrEngineNotImplemented
}

func (e *Engine) VirtualizationAttach(context.Context, string, bool, bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	return nil, nil, nil, types.ErrEngineNotImplemented
}

func (e *Engine) VirtualizationResize(context.Context, string, uint, uint) error {
	return types.ErrEngineNotImplemented
}

func (e *Engine) VirtualizationWait(context.Context, string, string) (*enginetypes.VirtualizationWaitResult, error) {
	return nil, types.ErrEngineNotImplemented
}

func (e *Engine) VirtualizationUpdateResource(context.Context, string, resourcetypes.Resources) error {
	return types.ErrEngineNotImplemented
}
