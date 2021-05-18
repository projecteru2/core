package systemd

import (
	"context"
	"encoding/json"

	"github.com/projecteru2/core/engine/docker"
	enginetypes "github.com/projecteru2/core/engine/types"
)

// VirtualizationCreate create a workload
func (e *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) { // nolintlint
	rArgs := &docker.RawArgs{StorageOpt: map[string]string{}}
	if len(opts.RawArgs) > 0 {
		if err := json.Unmarshal(opts.RawArgs, rArgs); err != nil {
			return nil, err
		}
	}
	rArgs.Runtime = e.config.Systemd.Runtime
	b, err := json.Marshal(rArgs)
	if err != nil {
		return nil, err
	}
	opts.RawArgs = b
	return e.API.VirtualizationCreate(ctx, opts)
}
