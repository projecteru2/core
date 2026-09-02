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

type reallocRepair func()

func (c *Calcium) ReallocResource(ctx context.Context, opts *types.ReallocOptions) (err error) {
	logger := log.WithFunc("calcium.ReallocResource").WithField("opts", opts)
	logger.Infof(ctx, "realloc workload %+v with options %+v", opts.ID, opts.Resources)
	workload, err := c.GetWorkload(ctx, opts.ID)
	if err != nil {
		return err
	}
	node, err := c.store.GetNode(ctx, workload.Nodename)
	if err != nil {
		return err
	}
	var repair reallocRepair
	err = c.withWorkloadLocked(ctx, opts.ID, false, func(ctx context.Context, workload *types.Workload) error {
		repair, err = c.doReallocOnNode(ctx, node, workload, opts)
		logger.Error(ctx, err)
		return err
	})
	if repair != nil {
		repair()
	}
	return err
}

func (c *Calcium) doReallocOnNode(ctx context.Context, node *types.Node, workload *types.Workload, opts *types.ReallocOptions) (reallocRepair, error) {
	originWorkload := *workload

	var resources resourcetypes.Resources
	var deltaResources resourcetypes.Resources
	var engineParams resourcetypes.Resources
	var reallocated bool
	var runtimeUpdateAttempted bool

	logger := log.WithFunc("calcium.doReallocOnNode").WithField("opts", opts)
	nodeCommit, err := c.journal(ctx, logger, eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return nil, err
	}
	workloadCommit, err := c.journal(ctx, logger, eventWorkloadReallocated, workload.ID)
	if err != nil {
		nodeCommit()
		return nil, err
	}

	settled, err := utils.Txn(
		ctx,
		func(ctx context.Context) error {
			if err = c.withNodeKeyLocked(ctx, node, func(ctx context.Context) (err error) {
				engineParams, deltaResources, resources, err = c.rmgr.Realloc(ctx, workload.Nodename, workload.Resources, opts.Resources)
				return err
			}); err != nil {
				return err
			}
			reallocated = true
			logger.Debugf(ctx, "realloc workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
			workload.EngineParams = engineParams
			workload.Resources = resources
			return c.store.UpdateWorkload(ctx, workload)
		},
		func(ctx context.Context) error {
			runtimeUpdateAttempted = true
			return node.Engine.VirtualizationUpdateResource(ctx, opts.ID, engineParams)
		},
		func(ctx context.Context, _ bool) error {
			if !reallocated {
				return nil
			}
			c.remapped.Delete(workload.Nodename)
			var rollbackErr error
			if resourceErr := c.withNodeKeyLocked(ctx, node, func(ctx context.Context) error {
				return c.rmgr.RollbackRealloc(ctx, workload.Nodename, deltaResources)
			}); resourceErr != nil {
				rollbackErr = errors.Join(rollbackErr, resourceErr)
				logger.Errorf(ctx, rollbackErr, "failed to rollback workload %+v, resource args %+v, engine args %+v", workload.ID, litter.Sdump(resources), litter.Sdump(engineParams))
			}
			return errors.Join(rollbackErr, c.store.UpdateWorkload(ctx, &originWorkload))
		},
		c.config.GlobalTimeout,
	)
	needsRepair := settled && runtimeUpdateAttempted && err != nil
	if settled {
		nodeCommit()
		if !needsRepair {
			workloadCommit()
		}
	}
	switch {
	case needsRepair:
		return func() { c.repairRealloc(ctx, logger, workload.ID, workloadCommit) }, err
	case err != nil:
		return nil, err
	}
	c.invokePoolAsync(func() { c.RemapResourceAndLog(ctx, logger, node.Name) })
	return nil, nil
}

func (c *Calcium) repairRealloc(ctx context.Context, logger *log.Fields, workloadID string, commit func()) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.config.GlobalTimeout)
	defer cancel()

	if err := (&ReallocWorkloadHandler{calcium: c}).Handle(ctx, workloadID); err != nil {
		logger.Error(ctx, err, "repair realloc failed")
		return
	}
	commit()
}
