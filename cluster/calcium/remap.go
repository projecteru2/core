package calcium

import (
	"context"
	"fmt"
	"hash/fnv"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

// RemapResourceAndLog remaps node resources after a binding change and logs the outcome.
func (c *Calcium) RemapResourceAndLog(ctx context.Context, logger *log.Fields, node *types.Node) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.config.GlobalTimeout)
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
	engineParamsMap, workloads, err := c.computeRemap(ctx, node)
	if err != nil || len(engineParamsMap) == 0 {
		return err
	}

	commit, err := c.journal(ctx, logger, eventNodeRemapped, node.Name)
	if err != nil {
		return err
	}
	if err = c.applyRemap(ctx, logger, node.Name, workloads, engineParamsMap); err != nil {
		return err
	}
	commit()
	return nil
}

func (c *Calcium) computeRemap(ctx context.Context, node *types.Node) (map[string]resourcetypes.Resources, []*types.Workload, error) {
	workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
	if err != nil {
		return nil, nil, err
	}
	engineParamsMap, err := c.rmgr.Remap(ctx, node.Name, workloads)
	if err != nil {
		return nil, nil, err
	}
	return engineParamsMap, workloads, nil
}

func (c *Calcium) applyRemap(ctx context.Context, logger *log.Fields, nodename string, workloads []*types.Workload, engineParamsMap map[string]resourcetypes.Resources) error {
	var errList []error
	memo, _ := c.remapped.Load(nodename)
	recorded, _ := memo.(map[string]uint64)
	applied := make(map[string]uint64, len(engineParamsMap))
	for _, workload := range workloads {
		engineParams, ok := engineParamsMap[workload.ID]
		if !ok {
			continue
		}
		digest := hashEngineParams(engineParams)
		if prev, seen := recorded[workload.ID]; seen && prev == digest {
			applied[workload.ID] = digest
			continue
		}
		logger.Infof(ctx, "remap workload ID %+v", workload.ID)
		switch err := workload.Engine.VirtualizationUpdateResource(ctx, workload.ID, engineParams); {
		case errors.Is(err, types.ErrWorkloadNotExists):
			logger.Debugf(ctx, "workload %s is gone, skip remap: %+v", workload.ID, err)
		case errors.Is(err, types.ErrEngineNotImplemented):
			logger.Warnf(ctx, "skip remap of workload %s: %+v", workload.ID, err)
		case err != nil:
			logger.Error(ctx, err)
			errList = append(errList, err)
		default:
			applied[workload.ID] = digest
		}
	}
	c.remapped.Store(nodename, applied)
	return errors.Join(errList...)
}

func hashEngineParams(params resourcetypes.Resources) uint64 {
	digest := fnv.New64a()
	_, _ = fmt.Fprintf(digest, "%v", params)
	return digest.Sum64()
}
