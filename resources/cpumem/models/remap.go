package models

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/cpumem/types"
)

// GetRemapArgs .
func (c *CPUMem) GetRemapArgs(ctx context.Context, node string, workloadResourceMap *types.WorkloadResourceArgsMap) (map[string]*types.EngineArgs, error) {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		log.Errorf(ctx, err, "[GetRemapArgs] failed to get resource info of node %v", node)
		return nil, err
	}
	availableNodeResource := resourceInfo.GetAvailableResource()

	shareCPUMap := types.CPUMap{}
	for cpu, pieces := range availableNodeResource.CPUMap {
		if pieces >= c.Config.Scheduler.ShareBase {
			shareCPUMap[cpu] = c.Config.Scheduler.ShareBase
		}
	}

	if len(shareCPUMap) == 0 {
		for cpu := range resourceInfo.Capacity.CPUMap {
			shareCPUMap[cpu] = c.Config.Scheduler.ShareBase
		}
	}

	engineArgsMap := map[string]*types.EngineArgs{}

	if workloadResourceMap == nil {
		return engineArgsMap, nil
	}

	for workloadID, workloadResourceArgs := range *workloadResourceMap {
		// only process workloads without cpu binding
		if len(workloadResourceArgs.CPUMap) == 0 {
			engineArgsMap[workloadID] = &types.EngineArgs{
				CPU:      workloadResourceArgs.CPULimit,
				CPUMap:   shareCPUMap,
				NUMANode: "",
				Memory:   workloadResourceArgs.MemoryLimit,
				Remap:    true,
			}
		}
	}
	return engineArgsMap, nil
}
