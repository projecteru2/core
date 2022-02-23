package models

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/cpumem/types"
)

// GetRemapArgs .
func (c *CPUMem) GetRemapArgs(ctx context.Context, node string, workloadResourceMap *types.WorkloadResourceArgsMap) (map[string]*types.EngineArgs, error) {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[GetRemapArgs] failed to get resource info of node %v, err: %v", node, err)
		return nil, err
	}
	availableNodeResource := resourceInfo.GetAvailableResource()

	shareCPUMap := types.CPUMap{}
	for cpu, pieces := range availableNodeResource.CPUMap {
		if pieces >= c.config.Scheduler.ShareBase {
			shareCPUMap[cpu] = c.config.Scheduler.ShareBase
		}
	}

	if len(shareCPUMap) == 0 {
		for cpu := range resourceInfo.Capacity.CPUMap {
			shareCPUMap[cpu] = c.config.Scheduler.ShareBase
		}
	}

	engineArgsMap := map[string]*types.EngineArgs{}

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
