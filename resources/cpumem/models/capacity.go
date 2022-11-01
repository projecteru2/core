package models

import (
	"context"
	"math"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/cpumem/schedule"
	"github.com/projecteru2/core/resources/cpumem/types"
	"github.com/projecteru2/core/utils"
)

// GetNodesDeployCapacity .
func (c *CPUMem) GetNodesDeployCapacity(ctx context.Context, nodes []string, opts *types.WorkloadResourceOpts) (map[string]*types.NodeCapacityInfo, int, error) {
	if err := opts.Validate(); err != nil {
		log.Errorf(ctx, err, "[GetNodesDeployCapacity] invalid resource opts %+v", opts)
		return nil, 0, err
	}

	capacityInfoMap := map[string]*types.NodeCapacityInfo{}
	total := 0
	for _, node := range nodes {
		resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
		if err != nil {
			log.Errorf(ctx, err, "[GetNodesDeployCapacity] failed to get resource info of node %+v", node)
			return nil, 0, err
		}
		capacityInfo := c.doGetNodeCapacityInfo(node, resourceInfo, opts)
		if capacityInfo.Capacity > 0 {
			capacityInfoMap[node] = capacityInfo
			if total == math.MaxInt || capacityInfo.Capacity == math.MaxInt {
				total = math.MaxInt
			} else {
				total += capacityInfo.Capacity
			}
		}
	}

	return capacityInfoMap, total, nil
}

func (c *CPUMem) doGetNodeCapacityInfo(node string, resourceInfo *types.NodeResourceInfo, opts *types.WorkloadResourceOpts) *types.NodeCapacityInfo {
	availableResourceArgs := resourceInfo.GetAvailableResource()

	capacityInfo := &types.NodeCapacityInfo{
		Node:   node,
		Weight: 1,
	}

	// if cpu-bind is not required, then returns capacity by memory
	if !opts.CPUBind {
		// check if cpu is enough
		if opts.CPURequest > float64(len(resourceInfo.Capacity.CPUMap)) {
			return capacityInfo
		}

		// calculate by memory request
		if opts.MemRequest == 0 {
			capacityInfo.Capacity = math.MaxInt
			capacityInfo.Rate = 0
		} else {
			capacityInfo.Capacity = int(availableResourceArgs.Memory / opts.MemRequest)
			capacityInfo.Rate = utils.AdvancedDivide(float64(opts.MemRequest), float64(resourceInfo.Capacity.Memory))
		}
		capacityInfo.Usage = utils.AdvancedDivide(float64(resourceInfo.Usage.Memory), float64(resourceInfo.Capacity.Memory))

		return capacityInfo
	}

	// if cpu-bind is required, then returns capacity by cpu scheduling
	cpuPlans := schedule.GetCPUPlans(resourceInfo, nil, c.Config.Scheduler.ShareBase, c.Config.Scheduler.MaxShare, opts)
	capacityInfo.Capacity = len(cpuPlans)
	capacityInfo.Usage = utils.AdvancedDivide(resourceInfo.Usage.CPU, resourceInfo.Capacity.CPU)
	capacityInfo.Rate = utils.AdvancedDivide(opts.CPURequest, resourceInfo.Capacity.CPU)
	capacityInfo.Weight = 100

	return capacityInfo
}
