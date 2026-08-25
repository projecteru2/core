package calcium

import (
	"context"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) ReallocResource(ctx context.Context, opts *types.ReallocOptions) (err error) {
	logger := log.WithFunc("calcium.ReallocResource").WithField("opts", opts)
	logger.Infof(ctx, "realloc workload %+v with options %+v", opts.ID, opts.Resources)
	workload, err := c.GetWorkload(ctx, opts.ID)
	if err != nil {
		return err
	}
	return c.withNodePodLocked(ctx, workload.Nodename, func(ctx context.Context, node *types.Node) error {
		return c.withWorkloadLocked(ctx, opts.ID, false, func(ctx context.Context, workload *types.Workload) error {
			err := c.doReallocOnNode(ctx, node, workload, *workload, opts)
			logger.Error(ctx, err)
			return err
		})
	})
}

func (c *Calcium) doReallocOnNode(ctx context.Context, node *types.Node, workload *types.Workload, originWorkload types.Workload, opts *types.ReallocOptions) error {
	var resources resourcetypes.Resources
	var deltaResources resourcetypes.Resources
	var engineParams resourcetypes.Resources
	var err error

	logger := log.WithFunc("calcium.doReallocOnNode").WithField("opts", opts)
	nodeCommit, err := c.wal.Log(eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	defer func() {
		if commitErr := nodeCommit(); commitErr != nil {
			logger.Errorf(ctx, commitErr, "commit wal failed: %s", eventWorkloadResourceAllocated)
		}
	}()
	workloadCommit, err := c.wal.Log(eventWorkloadReallocated, workload.ID)
	if err != nil {
		return err
	}
	defer func() {
		if commitErr := workloadCommit(); commitErr != nil {
			logger.Errorf(ctx, commitErr, "commit wal failed: %s", eventWorkloadReallocated)
		}
	}()

	err = utils.Txn(
		ctx,
		func(ctx context.Context) error {
			// Realloc mutates node resource meta in the resource plugin
			engineParams, deltaResources, resources, err = c.rmgr.Realloc(ctx, workload.Nodename, workload.Resources, opts.Resources)
			if err != nil {
				return err
			}
			logger.Debugf(ctx, "realloc workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
			workload.EngineParams = engineParams
			workload.Resources = resources
			return c.store.UpdateWorkload(ctx, workload)
		},
		func(ctx context.Context) error {
			return node.Engine.VirtualizationUpdateResource(ctx, opts.ID, engineParams)
		},
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			if rollbackErr := c.rmgr.RollbackRealloc(ctx, workload.Nodename, deltaResources); rollbackErr != nil {
				logger.Errorf(ctx, rollbackErr, "failed to rollback workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
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
