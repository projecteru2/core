package models

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/cpumem/types"
	coretypes "github.com/projecteru2/core/types"
)

// AddNode .
func (c *CPUMem) AddNode(ctx context.Context, node string, resourceOpts *types.NodeResourceOpts) (*types.NodeResourceInfo, error) {
	if _, err := c.doGetNodeResourceInfo(ctx, node); err != nil {
		if !errors.Is(err, coretypes.ErrBadCount) {
			log.Errorf(ctx, err, "[AddNode] failed to get resource info of node %+v", node)
			return nil, err
		}
	} else {
		return nil, types.ErrNodeExists
	}

	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:        float64(len(resourceOpts.CPUMap)),
			CPUMap:     resourceOpts.CPUMap,
			Memory:     resourceOpts.Memory,
			NUMAMemory: resourceOpts.NUMAMemory,
			NUMA:       resourceOpts.NUMA,
		},
		Usage: &types.NodeResourceArgs{
			CPU:        0,
			CPUMap:     types.CPUMap{},
			Memory:     0,
			NUMAMemory: types.NUMAMemory{},
		},
	}

	// if NUMA is set but NUMAMemory is not set
	// then divide memory equally according to the number of numa nodes
	if len(resourceOpts.NUMA) > 0 && resourceOpts.NUMAMemory == nil {
		averageMemory := resourceOpts.Memory / int64(len(resourceOpts.NUMA))
		resourceInfo.Capacity.NUMAMemory = types.NUMAMemory{}
		for _, numaNodeID := range resourceOpts.NUMA {
			resourceInfo.Capacity.NUMAMemory[numaNodeID] = averageMemory
		}
	}

	for cpu := range resourceOpts.CPUMap {
		resourceInfo.Usage.CPUMap[cpu] = 0
	}

	for numaNodeID := range resourceOpts.NUMA {
		resourceInfo.Usage.NUMAMemory[numaNodeID] = 0
	}

	return resourceInfo, c.doSetNodeResourceInfo(ctx, node, resourceInfo)
}

// RemoveNode .
func (c *CPUMem) RemoveNode(ctx context.Context, node string) error {
	if _, err := c.store.Delete(ctx, fmt.Sprintf(NodeResourceInfoKey, node)); err != nil {
		log.Errorf(ctx, err, "[doSetNodeResourceInfo] faield to delete node %+v", node)
		return err
	}
	return nil
}
