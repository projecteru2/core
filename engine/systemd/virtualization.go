package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// VirtualizationResourceRemap .
func (e *Engine) VirtualizationResourceRemap(ctx context.Context, opts *enginetypes.VirtualizationRemapOptions) (ch <-chan enginetypes.VirtualizationRemapMessage, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationLogs fetches service logs
func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout io.ReadCloser, stderr io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationAttach attaches a service's stdio
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (stdout, stderr io.ReadCloser, writer io.WriteCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationResize resizes a terminal window
func (e *Engine) VirtualizationResize(ctx context.Context, ID string, height, width uint) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationWait waits for service finishing
func (e *Engine) VirtualizationWait(ctx context.Context, ID, state string) (res *enginetypes.VirtualizationWaitResult, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// VirtualizationUpdateResource updates service resource limits
func (e *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) (err error) {
	err = types.ErrEngineNotImplemented
	return
}
