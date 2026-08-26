package calcium

import (
	"context"

	"github.com/cockroachdb/errors"
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
	var reallocated bool
	var rollbackComplete bool

	logger := log.WithFunc("calcium.doReallocOnNode").WithField("opts", opts)
	nodeCommit, err := c.journal(ctx, logger, eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	workloadCommit, err := c.journal(ctx, logger, eventWorkloadReallocated, workload.ID)
	if err != nil {
		nodeCommit()
		return err
	}

	err = utils.Txn(
		ctx,
		func(ctx context.Context) error {
			// Realloc mutates node resource meta in the resource plugin
			engineParams, deltaResources, resources, err = c.rmgr.Realloc(ctx, workload.Nodename, workload.Resources, opts.Resources)
			if err != nil {
				return err
			}
			reallocated = true
			logger.Debugf(ctx, "realloc workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
			workload.EngineParams = engineParams
			workload.Resources = resources
			return c.store.UpdateWorkload(ctx, workload)
		},
		func(ctx context.Context) error {
			return node.Engine.VirtualizationUpdateResource(ctx, opts.ID, engineParams)
		},
		func(ctx context.Context, _ bool) error {
			if !reallocated {
				rollbackComplete = true
				return nil
			}
			var rollbackErr error
			if resourceErr := c.rmgr.RollbackRealloc(ctx, workload.Nodename, deltaResources); resourceErr != nil {
				rollbackErr = errors.Join(rollbackErr, resourceErr)
				logger.Errorf(ctx, rollbackErr, "failed to rollback workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
			}
			rollbackErr = errors.Join(rollbackErr, c.store.UpdateWorkload(ctx, &originWorkload))
			rollbackComplete = rollbackErr == nil
			return rollbackErr
		},
		c.config.GlobalTimeout,
	)
	if err == nil || rollbackComplete {
		nodeCommit()
		workloadCommit()
	}
	if err != nil {
		return err
	}
	_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })
	return nil
}
