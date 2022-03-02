package models

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/cpumem/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestAddNode(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 1, 1, 100)

	node := "node"
	resourceOpts := &types.NodeResourceOpts{
		CPUMap: types.CPUMap{"0": 100, "1": 100},
		Memory: 4 * units.GiB,
		NUMA:   types.NUMA{"0": "0", "1": "1"},
	}

	// existent node
	_, err := cpuMem.AddNode(ctx, nodes[0], resourceOpts)
	assert.Equal(t, err, types.ErrNodeExists)

	// normal case
	resourceInfo, err := cpuMem.AddNode(ctx, node, resourceOpts)
	assert.Nil(t, err)
	assert.Equal(t, resourceInfo, &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:        2,
			CPUMap:     types.CPUMap{"0": 100, "1": 100},
			Memory:     4 * units.GiB,
			NUMAMemory: types.NUMAMemory{"0": 2 * units.GiB, "1": 2 * units.GiB},
			NUMA:       types.NUMA{"0": "0", "1": "1"},
		},
		Usage: &types.NodeResourceArgs{
			CPU:        0,
			CPUMap:     types.CPUMap{"0": 0, "1": 0},
			Memory:     0,
			NUMAMemory: types.NUMAMemory{"0": 0, "1": 0},
			NUMA:       types.NUMA{},
		},
	})
}

func TestRemoveNode(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 1, 1, 100)

	assert.Nil(t, cpuMem.RemoveNode(ctx, nodes[0]))
	_, _, err := cpuMem.GetNodeResourceInfo(ctx, nodes[0], nil, false)
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	assert.Nil(t, cpuMem.RemoveNode(ctx, "xxx"))
}
