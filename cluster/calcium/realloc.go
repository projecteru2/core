package calcium

import (
	"context"

	"github.com/sanity-io/litter"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// ReallocResource updates workload resource dynamically
func (c *Calcium) ReallocResource(ctx context.Context, opts *types.ReallocOptions) (err error) {
	logger := log.WithFunc("calcium.ReallocResource").WithField("opts", opts)
	logger.Infof(ctx, "realloc workload %+v with options %+v", opts.ID, opts.Resources)
	workload, err := c.GetWorkload(ctx, opts.ID)
	if err != nil {
		return
	}
	// copy origin workload
	originWorkload := *workload
	return c.withNodePodLocked(ctx, workload.Nodename, func(ctx context.Context, node *types.Node) error {
		return c.withWorkloadLocked(ctx, opts.ID, func(ctx context.Context, workload *types.Workload) error {
			err := c.doReallocOnNode(ctx, node, workload, originWorkload, opts)
			logger.Error(ctx, err)
			return err
		})
	})
}

func (c *Calcium) doReallocOnNode(ctx context.Context, node *types.Node, workload *types.Workload, originWorkload types.Workload, opts *types.ReallocOptions) error {
	var resources *types.Resources
	var deltaResources *types.Resources
	var engineParams *types.Resources
	var err error

	logger := log.WithFunc("calcium.doReallocOnNode").WithField("opts", opts)
	err = utils.Txn(
		ctx,
		// if: update workload resource
		func(ctx context.Context) error {
			// note here will change the node resource meta (stored in resource plugin)
			// todo: add wal here
			engineParams, deltaResources, resources, err = c.rmgr.Realloc(ctx, workload.Nodename, workload.Resources, opts.Resources)
			if err != nil {
				return err
			}
			logger.Debugf(ctx, "realloc workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
			workload.EngineParams = engineParams
			workload.Resources = resources
			return c.store.UpdateWorkload(ctx, workload)
		},
		// then: update virtualization
		func(ctx context.Context) error {
			return node.Engine.VirtualizationUpdateResource(ctx, opts.ID, &enginetypes.VirtualizationResource{EngineParams: engineParams})
		},
		// rollback: revert the resource changes and rollback workload meta
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			if err := c.rmgr.RollbackRealloc(ctx, workload.Nodename, deltaResources); err != nil {
				logger.Errorf(ctx, err, "failed to rollback workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
				// don't return here, so the node resource can still be fixed
			}
			return c.store.UpdateWorkload(ctx, &originWorkload)
		},
		c.config.GlobalTimeout,
	)
	if err != nil {
		return err
	}
	_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })
	return nil
}
