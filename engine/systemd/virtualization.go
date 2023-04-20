package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

// VirtualizationLogs fetches service logs
func (e *Engine) VirtualizationLogs(_ context.Context, _ *enginetypes.VirtualizationLogStreamOptions) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationAttach attaches a service's stdio
func (e *Engine) VirtualizationAttach(_ context.Context, _ string, _, _ bool) (stdout, stderr io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationResize resizes a terminal window
func (e *Engine) VirtualizationResize(_ context.Context, _ string, _, _ uint) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationWait waits for service finishing
func (e *Engine) VirtualizationWait(_ context.Context, _, _ string) (res *enginetypes.VirtualizationWaitResult, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationUpdateResource updates service resource limits
func (e *Engine) VirtualizationUpdateResource(context.Context, string, resourcetypes.Resources) (err error) {
	err = types.ErrEngineNotImplemented
	return
}
