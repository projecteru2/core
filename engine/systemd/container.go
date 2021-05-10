package systemd

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
)

// VirtualizationCreate create a workload
func (e *systemdEngine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) { // nolintlint
	opts.Runtime = e.config.Systemd.Runtime
	return e.API.VirtualizationCreate(ctx, opts)
}
