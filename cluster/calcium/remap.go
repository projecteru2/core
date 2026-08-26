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

	errList := make([]error, 0, len(engineParamsMap))
	for workloadID, engineParams := range engineParamsMap {
		remap := &workloadRemap{ID: workloadID, EngineParams: engineParams}
		commit, journalErr := c.journal(ctx, logger, eventWorkloadRemapped, remap)
		if journalErr != nil {
			errList = append(errList, journalErr)
			continue
		}
		if remapErr := c.applyWorkloadRemap(ctx, logger, remap); remapErr != nil {
			errList = append(errList, remapErr)
			continue
		}
		commit()
	}
	return errors.Join(errList...)
}

func (c *Calcium) applyWorkloadRemap(ctx context.Context, logger *log.Fields, remap *workloadRemap) error {
	workload, err := getWorkloadIfExists(ctx, c, remap.ID)
	if err != nil || workload == nil {
		return err
	}

	updatedWorkload := *workload
	updatedWorkload.EngineParams = remap.EngineParams
	if err = c.store.UpdateWorkload(ctx, &updatedWorkload); err != nil {
		return err
	}

	logger.Infof(ctx, "remap workload ID %+v", remap.ID)
	switch err = workload.Engine.VirtualizationUpdateResource(ctx, remap.ID, remap.EngineParams); {
	case errors.Is(err, types.ErrWorkloadNotExists), errors.Is(err, types.ErrEngineNotImplemented):
		logger.Warnf(ctx, "skip remap of workload %s: %+v", remap.ID, err)
		return nil
	case err != nil:
		logger.Error(ctx, err)
		return err
	}
	return nil
}
