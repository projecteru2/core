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
	run := func() {
		defer wg.Done()
		if err = c.withWorkloadLocked(ctx, ID, opts.IgnoreLock, func(ctx context.Context, workload *types.Workload) error {
			msg, err = workload.RawEngine(ctx, opts)
			return err
		}); err == nil {
			logger.Infof(ctx, "workload %s raw engine result: %+v", ID, msg)
		}
	}
	if invokeErr := c.pool.Invoke(run); invokeErr != nil {
		run()
	}
	wg.Wait()

	logger.Error(ctx, err)
	return msg, err
}
