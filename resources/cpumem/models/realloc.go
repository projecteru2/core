package models

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/cpumem/schedule"
	"github.com/projecteru2/core/resources/cpumem/types"
)

// GetReallocArgs .
func (c *CPUMem) GetReallocArgs(ctx context.Context, node string, originResourceArgs *types.WorkloadResourceArgs, resourceOpts *types.WorkloadResourceOpts) (*types.EngineArgs, *types.WorkloadResourceArgs, *types.WorkloadResourceArgs, error) {
	if resourceOpts.KeepCPUBind {
		resourceOpts.CPUBind = len(originResourceArgs.CPUMap) > 0
	}

	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[GetReallocArgs] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, nil, err
	}

	// put resources back into the resource pool
	resourceInfo.Usage.Sub(&types.NodeResourceArgs{
		CPU:        originResourceArgs.CPURequest,
		CPUMap:     originResourceArgs.CPUMap,
		Memory:     originResourceArgs.MemoryRequest,
		NUMAMemory: originResourceArgs.NUMAMemory,
	})

	finalResourceOpts := &types.WorkloadResourceOpts{
		CPUBind:    resourceOpts.CPUBind,
		CPURequest: resourceOpts.CPURequest + originResourceArgs.CPURequest,
		CPULimit:   resourceOpts.CPULimit + originResourceArgs.CPULimit,
		MemRequest: resourceOpts.MemRequest + originResourceArgs.MemoryRequest,
		MemLimit:   resourceOpts.MemLimit + originResourceArgs.MemoryLimit,
	}

	if err = finalResourceOpts.Validate(); err != nil {
		return nil, nil, nil, err
	}

	// if cpu was specified before, try to ensure cpu affinity
	var cpuMap types.CPUMap
	var numaNodeID string
	var numaMemory types.NUMAMemory

	if resourceOpts.CPUBind {
		cpuPlans := schedule.GetCPUPlans(resourceInfo, originResourceArgs.CPUMap, c.config.Scheduler.ShareBase, c.config.Scheduler.MaxShare, finalResourceOpts)
		if len(cpuPlans) == 0 {
			return nil, nil, nil, types.ErrInsufficientResource
		}

		cpuPlan := cpuPlans[0]
		cpuMap = cpuPlan.CPUMap
		numaNodeID = cpuPlan.NUMANode
		if len(numaNodeID) > 0 {
			numaMemory = types.NUMAMemory{numaNodeID: finalResourceOpts.MemRequest}
		}
	} else {
		if _, _, err = c.doAllocByMemory(resourceInfo, 1, finalResourceOpts); err != nil {
			return nil, nil, nil, err
		}
	}

	engineArgs := &types.EngineArgs{
		CPU:      finalResourceOpts.CPULimit,
		CPUMap:   cpuMap,
		NUMANode: numaNodeID,
		Memory:   finalResourceOpts.MemLimit,
	}

	finalWorkloadResourceArgs := &types.WorkloadResourceArgs{
		CPURequest:    finalResourceOpts.CPURequest,
		CPULimit:      finalResourceOpts.CPULimit,
		MemoryRequest: finalResourceOpts.MemRequest,
		MemoryLimit:   finalResourceOpts.MemLimit,
		CPUMap:        cpuMap,
		NUMAMemory:    numaMemory,
		NUMANode:      numaNodeID,
	}

	deltaWorkloadResourceArgs := finalWorkloadResourceArgs.DeepCopy()
	deltaWorkloadResourceArgs.Sub(originResourceArgs)

	return engineArgs, deltaWorkloadResourceArgs, finalWorkloadResourceArgs, nil
}
