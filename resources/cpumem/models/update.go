package models

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/cpumem/types"
)

// UpdateNodeResourceUsage .
func (c *CPUMem) UpdateNodeResourceUsage(ctx context.Context, node string, resourceArgsList []*types.WorkloadResourceArgs, incr bool) error {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[UpdateNodeResourceUsage] failed to get resource info of node %v, err: %v", node, err)
		return err
	}

	for _, resourceArgs := range resourceArgsList {
		nodeResourceArgs := &types.NodeResourceArgs{
			CPU:        resourceArgs.CPURequest,
			CPUMap:     resourceArgs.CPUMap,
			Memory:     resourceArgs.MemoryRequest,
			NUMAMemory: resourceArgs.NUMAMemory,
		}

		if incr {
			resourceInfo.Usage.Add(nodeResourceArgs)
		} else {
			resourceInfo.Usage.Sub(nodeResourceArgs)
		}
	}

	return c.doSetNodeResourceInfo(ctx, node, resourceInfo)
}

// UpdateNodeResourceCapacity .
func (c *CPUMem) UpdateNodeResourceCapacity(ctx context.Context, node string, resourceOpts *types.NodeResourceOpts, incr bool) error {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[UpdateNodeResourceCapacity] failed to get resource info of node %v, err: %v", node, err)
		return err
	}

	resourceArgs := &types.NodeResourceArgs{
		CPUMap:     resourceOpts.CPUMap,
		Memory:     resourceOpts.Memory,
		NUMAMemory: resourceOpts.NUMAMemory,
	}

	if len(resourceOpts.NUMA) > 0 {
		resourceInfo.Capacity.NUMA = resourceOpts.NUMA
	}

	if incr {
		resourceInfo.Capacity.Add(resourceArgs)
	} else {
		resourceInfo.Capacity.Sub(resourceArgs)
	}

	// add new cpu
	for cpu := range resourceInfo.Capacity.CPUMap {
		_, ok := resourceInfo.Usage.CPUMap[cpu]
		if !ok {
			resourceInfo.Usage.CPUMap[cpu] = 0
			continue
		}
	}

	// delete cpus with no pieces
	resourceInfo.RemoveEmptyCores()

	return c.doSetNodeResourceInfo(ctx, node, resourceInfo)
}
