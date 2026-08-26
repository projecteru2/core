package calcium

import (
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// RemapResourceAndLog remaps node resources after a binding change and logs the outcome.
func (c *Calcium) RemapResourceAndLog(ctx context.Context, logger *log.Fields, node *types.Node) {
	ctx, cancel := context.WithTimeout(utils.NewInheritCtx(ctx), c.config.GlobalTimeout)
	defer cancel()

	err := c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
		return c.doRemapResource(ctx, logger, node)
	})
	if err != nil {
		logger.Error(ctx, err, "remap node failed")
	}
}

// the caller must hold the node lock
func (c *Calcium) doRemapResource(ctx context.Context, logger *log.Fields, node *types.Node) error {
	workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
	if err != nil {
		return err
	}

	engineParamsMap, err := c.rmgr.Remap(ctx, node.Name, workloads)
	if err != nil {
		return err
	}

	for workloadID, engineParams := range engineParamsMap {
		logger.Infof(ctx, "remap workload ID %+v", workloadID)
		switch updateErr := node.Engine.VirtualizationUpdateResource(ctx, workloadID, engineParams); {
		case errors.Is(updateErr, types.ErrWorkloadNotExists):
			logger.Warnf(ctx, "skip remap of workload %s: %+v", workloadID, updateErr)
		case updateErr != nil:
			logger.Error(ctx, updateErr)
		}
	}
	return nil
}
