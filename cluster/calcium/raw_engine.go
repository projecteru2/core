package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) RawEngine(ctx context.Context, opts *types.RawEngineOptions) (msg *types.RawEngineMessage, err error) {
	ID := opts.ID
	logger := log.WithFunc("calcium.RawEngine").WithField("ID", opts.ID)
	if err = c.withWorkloadLocked(ctx, ID, opts.IgnoreLock, func(ctx context.Context, workload *types.Workload) error {
		msg, err = workload.RawEngine(ctx, opts)
		return err
	}); err == nil {
		logger.Infof(ctx, "workload %s raw engine result: %+v", ID, msg)
	}

	logger.Error(ctx, err)
	return msg, err
}
