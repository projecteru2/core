package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) RawEngine(ctx context.Context, opts *types.RawEngineOptions) (msg *types.RawEngineMessage, err error) {
	ID := opts.ID
	logger := log.WithFunc("calcium.RawEngine").WithField("ID", opts.ID)
	var wg sync.WaitGroup
	wg.Add(1)
	_ = c.pool.Invoke(func() {
		defer wg.Done()
		if opts.IgnoreLock {
			var workload *types.Workload
			if workload, err = c.store.GetWorkload(ctx, ID); err != nil {
				return
			}
			msg, err = workload.RawEngine(ctx, opts)
		} else {
			err = c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
				msg, err = workload.RawEngine(ctx, opts)
				return err
			})
		}

		if err == nil {
			logger.Infof(ctx, "Workload %s", ID)
			logger.Infof(ctx, "%+v", msg)
		}
	})
	wg.Wait()

	logger.Error(ctx, err)
	return
}
