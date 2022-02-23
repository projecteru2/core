package models

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/cpumem/schedule"
	"github.com/projecteru2/core/resources/cpumem/types"
)

// GetDeployArgs .
func (c *CPUMem) GetDeployArgs(ctx context.Context, node string, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	if err := opts.Validate(); err != nil {
		logrus.Errorf("[GetDeployArgs] invalid resource opts %+v, err: %v", opts, err)
		return nil, nil, err
	}

	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[GetDeployArgs] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, err
	}

	if !opts.CPUBind {
		return c.doAllocByMemory(resourceInfo, deployCount, opts)
	}

	return c.doAllocByCPU(resourceInfo, deployCount, opts)
}

func (c *CPUMem) doAllocByMemory(resourceInfo *types.NodeResourceInfo, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	if opts.CPURequest > float64(len(resourceInfo.Capacity.CPUMap)) {
		return nil, nil, types.ErrInsufficientCPU
	}

	availableResourceArgs := resourceInfo.GetAvailableResource()
	if opts.MemRequest > 0 && availableResourceArgs.Memory/opts.MemRequest < int64(deployCount) {
		return nil, nil, types.ErrInsufficientMem
	}

	resEngineArgs := []*types.EngineArgs{}
	resResourceArgs := []*types.WorkloadResourceArgs{}

	engineArgs := &types.EngineArgs{
		CPU:    opts.CPULimit,
		Memory: opts.MemLimit,
	}
	resourceArgs := &types.WorkloadResourceArgs{
		CPURequest:    opts.CPURequest,
		CPULimit:      opts.CPULimit,
		MemoryRequest: opts.MemRequest,
		MemoryLimit:   opts.MemLimit,
	}

	for len(resEngineArgs) < deployCount {
		resEngineArgs = append(resEngineArgs, engineArgs)
		resResourceArgs = append(resResourceArgs, resourceArgs)
	}
	return resEngineArgs, resResourceArgs, nil
}

func (c *CPUMem) doAllocByCPU(resourceInfo *types.NodeResourceInfo, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	cpuPlans := schedule.GetCPUPlans(resourceInfo, nil, c.config.Scheduler.ShareBase, c.config.Scheduler.MaxShare, opts)
	if len(cpuPlans) < deployCount {
		return nil, nil, types.ErrInsufficientResource
	}

	cpuPlans = cpuPlans[:deployCount]
	resEngineArgs := []*types.EngineArgs{}
	resResourceArgs := []*types.WorkloadResourceArgs{}

	for _, cpuPlan := range cpuPlans {
		resEngineArgs = append(resEngineArgs, &types.EngineArgs{
			CPU:      opts.CPULimit,
			CPUMap:   cpuPlan.CPUMap,
			NUMANode: cpuPlan.NUMANode,
			Memory:   opts.MemLimit,
		})

		resourceArgs := &types.WorkloadResourceArgs{
			CPURequest:    opts.CPURequest,
			CPULimit:      opts.CPULimit,
			MemoryRequest: opts.MemRequest,
			MemoryLimit:   opts.MemLimit,
			CPUMap:        cpuPlan.CPUMap,
			NUMANode:      cpuPlan.NUMANode,
		}
		if len(resourceArgs.NUMANode) > 0 {
			resourceArgs.NUMAMemory = types.NUMAMemory{resourceArgs.NUMANode: resourceArgs.MemoryRequest}
		}

		resResourceArgs = append(resResourceArgs, resourceArgs)
	}

	return resEngineArgs, resResourceArgs, nil
}
