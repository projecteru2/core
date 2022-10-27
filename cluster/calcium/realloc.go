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
	logger := log.WithField("Calcium", "ReallocResource").WithField("opts", opts)
	log.Infof(ctx, "[ReallocResource] realloc workload %v with options %v", opts.ID, opts.ResourceOpts)
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
	var resourceArgs map[string]types.WorkloadResourceArgs
	var deltaResourceArgs map[string]types.WorkloadResourceArgs
	var engineArgs types.EngineArgs
	var err error

	err = utils.Txn(
		ctx,
		// if: update workload resource
		func(ctx context.Context) error {
			// note here will change the node resource meta (stored in resource plugin)
			// todo: add wal here
			engineArgs, deltaResourceArgs, resourceArgs, err = c.rmgr.Realloc(ctx, workload.Nodename, workload.ResourceArgs, opts.ResourceOpts)
			if err != nil {
				return err
			}
			log.Debugf(ctx, "[doReallocOnNode] realloc workload %v, resource args %v, engine args %v", workload.ID, litter.Sdump(resourceArgs), litter.Sdump(engineArgs))
			workload.EngineArgs = engineArgs
			workload.ResourceArgs = resourceArgs
			return c.store.UpdateWorkload(ctx, workload)
		},
		// then: update virtualization
		func(ctx context.Context) error {
			return node.Engine.VirtualizationUpdateResource(ctx, opts.ID, &enginetypes.VirtualizationResource{EngineArgs: engineArgs})
		},
		// rollback: revert the resource changes and rollback workload meta
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			if err := c.rmgr.RollbackRealloc(ctx, workload.Nodename, deltaResourceArgs); err != nil {
				log.Errorf(ctx, err, "[doReallocOnNode] failed to rollback workload %v, resource args %v, engine args %v, err %v", workload.ID, litter.Sdump(resourceArgs), litter.Sdump(engineArgs), err)
				// don't return here, so the node resource can still be fixed
			}
			return c.store.UpdateWorkload(ctx, &originWorkload)
		},
		c.config.GlobalTimeout,
	)
	if err != nil {
		return err
	}
	_ = c.pool.Invoke(func() { c.doRemapResourceAndLog(ctx, log.WithField("Calcium", "doReallocOnNode"), node) })
	return nil
}
