package calcium

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
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
	return c.withNodeResourceLocked(ctx, workload.Nodename, func(ctx context.Context, node *types.Node) error {
		return c.withWorkloadLocked(ctx, opts.ID, func(ctx context.Context, workload *types.Workload) error {
			return logger.Err(ctx, c.doReallocOnNode(ctx, node, workload, &originWorkload, opts))
		})
	})
}

func (c *Calcium) doReallocOnNode(ctx context.Context, node *types.Node, workload *types.Workload, originWorkload *types.Workload, opts *types.ReallocOptions) error {
	var resourceArgs map[string]types.WorkloadResourceArgs
	var engineArgs types.EngineArgs
	var err error

	err = utils.Txn(
		ctx,
		// if: update workload resource
		func(ctx context.Context) error {
			// note here will change the node resource meta (stored in resource plugin)
			engineArgs, resourceArgs, err = c.resource.Realloc(ctx, workload.Nodename, workload.ResourceArgs, opts.ResourceOpts)
			if err != nil {
				return err
			}
			return node.Engine.VirtualizationUpdateResource(ctx, opts.ID, &enginetypes.VirtualizationResource{EngineArgs: engineArgs})
		},
		// then: update workload meta
		func(ctx context.Context) error {
			workload.EngineArgs = engineArgs
			workload.ResourceArgs = resourceArgs
			return c.store.UpdateWorkload(ctx, workload)
		},
		// rollback: revert the resource changes and rollback workload meta
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			_, _, err := c.resource.SetNodeResourceUsage(ctx, workload.Nodename, nil, nil, []map[string]types.WorkloadResourceArgs{resourceArgs}, true, resources.Decr)
			if err != nil {
				return errors.WithStack(err)
			}
			return errors.WithStack(c.store.UpdateWorkload(ctx, originWorkload))
		},
		c.config.GlobalTimeout,
	)
	if err != nil {
		return errors.WithStack(err)
	}
	go c.doRemapResourceAndLog(ctx, log.WithField("Calcium", "doReallocOnNode"), node)
	return nil
}
