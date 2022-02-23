package models

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/cpumem/types"
	"github.com/projecteru2/core/utils"
)

const NodeResourceInfoKey = "/resource/cpumem/%s"

// GetNodeResourceInfo .
func (c *CPUMem) GetNodeResourceInfo(ctx context.Context, node string, workloadResourceMap *types.WorkloadResourceArgsMap, fix bool) (*types.NodeResourceInfo, []string, error) {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		return nil, nil, err
	}

	diffs := []string{}

	totalResourceArgs := &types.WorkloadResourceArgs{
		CPUMap:     types.CPUMap{},
		NUMAMemory: types.NUMAMemory{},
	}
	for _, args := range *workloadResourceMap {
		totalResourceArgs.Add(args)
	}

	totalResourceArgs.CPURequest = utils.Round(totalResourceArgs.CPURequest)
	totalCPUUsage := utils.Round(resourceInfo.Usage.CPU)
	if totalResourceArgs.CPURequest != totalCPUUsage {
		diffs = append(diffs, fmt.Sprintf("node.CPUUsed != sum(workload.CPURequest): %.2f != %.2f", totalCPUUsage, totalResourceArgs.CPURequest))
	}

	for cpu := range resourceInfo.Capacity.CPUMap {
		if totalResourceArgs.CPUMap[cpu] != resourceInfo.Usage.CPUMap[cpu] {
			diffs = append(diffs, fmt.Sprintf("node.CPUMap[%v] != sum(workload.CPUMap[%v]): %v != %v", cpu, cpu, resourceInfo.Usage.CPUMap[cpu], totalResourceArgs.CPUMap[cpu]))
		}
	}

	for numaNodeID := range resourceInfo.Capacity.NUMAMemory {
		if totalResourceArgs.NUMAMemory[numaNodeID] != resourceInfo.Usage.NUMAMemory[numaNodeID] {
			diffs = append(diffs, fmt.Sprintf("node.NUMAMemory[%v] != sum(workload.NUMAMemory[%v]: %v != %v)", numaNodeID, numaNodeID, resourceInfo.Usage.NUMAMemory[numaNodeID], totalResourceArgs.NUMAMemory[numaNodeID]))
		}
	}

	if resourceInfo.Usage.Memory != totalResourceArgs.MemoryRequest {
		diffs = append(diffs, fmt.Sprintf("node.MemoryUsed != sum(workload.MemoryRequest): %d != %d", resourceInfo.Usage.Memory, totalResourceArgs.MemoryRequest))
	}

	if fix {
		resourceInfo.Usage = &types.NodeResourceArgs{
			CPU:        totalResourceArgs.CPURequest,
			CPUMap:     totalResourceArgs.CPUMap,
			Memory:     totalResourceArgs.MemoryRequest,
			NUMAMemory: totalResourceArgs.NUMAMemory,
		}
		if err = c.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
			logrus.Warnf("[GetNodeResourceInfo] failed to fix node resource, err: %v", err)
			diffs = append(diffs, "fix failed")
		}
	}

	return resourceInfo, diffs, nil
}

// calculateNodeResourceArgs priority: node resource opts > node resource args > workload resource args list
func (c *CPUMem) calculateNodeResourceArgs(origin *types.NodeResourceArgs, nodeResourceOpts *types.NodeResourceOpts, nodeResourceArgs *types.NodeResourceArgs, workloadResourceArgs []*types.WorkloadResourceArgs, delta bool, incr bool) (res *types.NodeResourceArgs) {
	if origin == nil || !delta {
		res = (&types.NodeResourceArgs{}).DeepCopy()
	} else {
		res = origin.DeepCopy()
	}

	if nodeResourceOpts != nil {
		nodeResourceArgs := &types.NodeResourceArgs{
			CPU:        float64(len(nodeResourceOpts.CPUMap)),
			CPUMap:     nodeResourceOpts.CPUMap,
			Memory:     nodeResourceOpts.Memory,
			NUMAMemory: nodeResourceOpts.NUMAMemory,
			NUMA:       nodeResourceOpts.NUMA,
		}

		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}
		return res
	}

	if nodeResourceArgs != nil {
		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}
		return res
	}

	for _, args := range workloadResourceArgs {
		nodeResourceArgs := &types.NodeResourceArgs{
			CPU:        args.CPURequest,
			CPUMap:     args.CPUMap,
			NUMAMemory: args.NUMAMemory,
			Memory:     args.MemoryRequest,
		}
		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}
	}
	return res
}

// SetNodeResourceUsage .
func (c *CPUMem) SetNodeResourceUsage(ctx context.Context, node string, nodeResourceOpts *types.NodeResourceOpts, nodeResourceArgs *types.NodeResourceArgs, workloadResourceArgs []*types.WorkloadResourceArgs, delta bool, incr bool) (before *types.NodeResourceArgs, after *types.NodeResourceArgs, err error) {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[SetNodeResourceInfo] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, err
	}

	before = resourceInfo.Usage.DeepCopy()
	resourceInfo.Usage = c.calculateNodeResourceArgs(resourceInfo.Usage, nodeResourceOpts, nodeResourceArgs, workloadResourceArgs, delta, incr)

	if err := c.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
		return nil, nil, err
	}
	return before, resourceInfo.Usage, nil
}

// SetNodeResourceCapacity .
func (c *CPUMem) SetNodeResourceCapacity(ctx context.Context, node string, nodeResourceOpts *types.NodeResourceOpts, nodeResourceArgs *types.NodeResourceArgs, delta bool, incr bool) (before *types.NodeResourceArgs, after *types.NodeResourceArgs, err error) {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[SetNodeResourceInfo] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, err
	}

	before = resourceInfo.Capacity.DeepCopy()
	if !delta {
		nodeResourceOpts.SkipEmpty(resourceInfo.Capacity)
	}

	resourceInfo.Capacity = c.calculateNodeResourceArgs(resourceInfo.Usage, nodeResourceOpts, nodeResourceArgs, nil, delta, incr)

	if err := c.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
		return nil, nil, err
	}
	return before, resourceInfo.Capacity, nil
}

// SetNodeResourceInfo .
func (c *CPUMem) SetNodeResourceInfo(ctx context.Context, node string, resourceCapacity *types.NodeResourceArgs, resourceUsage *types.NodeResourceArgs) error {
	resourceInfo, err := c.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[SetNodeResourceInfo] failed to get resource info of node %v, err: %v", node, err)
		return err
	}

	resourceInfo.Capacity = resourceCapacity
	resourceInfo.Usage = resourceUsage

	return c.doSetNodeResourceInfo(ctx, node, resourceInfo)
}

func (c *CPUMem) doGetNodeResourceInfo(ctx context.Context, node string) (*types.NodeResourceInfo, error) {
	resourceInfo := &types.NodeResourceInfo{}
	resp, err := c.store.GetOne(ctx, fmt.Sprintf(NodeResourceInfoKey, node))
	if err != nil {
		logrus.Errorf("[doGetNodeResourceInfo] failed to get node resource info of node %v, err: %v", node, err)
		return nil, err
	}
	if err = json.Unmarshal(resp.Value, resourceInfo); err != nil {
		logrus.Errorf("[doGetNodeResourceInfo] failed to unmarshal node resource info of node %v, err: %v", node, err)
		return nil, err
	}
	return resourceInfo, nil
}

func (c *CPUMem) doSetNodeResourceInfo(ctx context.Context, node string, resourceInfo *types.NodeResourceInfo) error {
	if err := resourceInfo.Validate(); err != nil {
		logrus.Errorf("[doSetNodeResourceInfo] invalid resource info %+v, err: %v", resourceInfo, err)
		return err
	}

	data, err := json.Marshal(resourceInfo)
	if err != nil {
		logrus.Errorf("[doSetNodeResourceInfo] faield to marshal resource info %+v, err: %v", resourceInfo, err)
		return err
	}

	if _, err = c.store.Put(ctx, fmt.Sprintf(NodeResourceInfoKey, node), string(data)); err != nil {
		logrus.Errorf("[doSetNodeResourceInfo] faield to put resource info %+v, err: %v", resourceInfo, err)
		return err
	}
	return nil
}
