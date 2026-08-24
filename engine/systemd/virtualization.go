package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

func (e *Engine) VirtualizationLogs(_ context.Context, _ *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return stdout, stderr, err
}

func (e *Engine) VirtualizationAttach(_ context.Context, _ string, _, _ bool) (stdout, stderr io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return stdout, stderr, writer, err
}

func (e *Engine) VirtualizationResize(_ context.Context, _ string, _, _ uint) (err error) {
	err = types.ErrEngineNotImplemented
	return err
}

func (e *Engine) VirtualizationWait(_ context.Context, _, _ string) (res *enginetypes.VirtualizationWaitResult, err error) {
	err = types.ErrEngineNotImplemented
	return res, err
}

func (e *Engine) VirtualizationUpdateResource(context.Context, string, resourcetypes.Resources) (err error) {
	err = types.ErrEngineNotImplemented
	return err
}
