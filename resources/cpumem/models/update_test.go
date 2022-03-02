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

func TestUpdateNodeResourceUsage(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	node := generateNodes(t, cpuMem, 1, 4, 4*units.GiB, 100)[0]

	resourceArgsList := []*types.WorkloadResourceArgs{
		{
			CPUMap:        types.CPUMap{"0": 50},
			CPURequest:    0.5,
			CPULimit:      0.5,
			MemoryRequest: 512 * units.MiB,
			MemoryLimit:   512 * units.MiB,
		},
	}

	// non-existent node
	err := cpuMem.UpdateNodeResourceUsage(ctx, "xxx", resourceArgsList, true)
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	// invalid resource info
	err = cpuMem.UpdateNodeResourceUsage(ctx, node, resourceArgsList, false)
	assert.True(t, errors.Is(err, types.ErrInvalidCPUMap))

	// normal case
	err = cpuMem.UpdateNodeResourceUsage(ctx, node, resourceArgsList, true)
	assert.Nil(t, err)

	resourceInfo, _, err := cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)
	assert.Equal(t, resourceInfo.Usage, &types.NodeResourceArgs{
		CPU:        0.5,
		CPUMap:     types.CPUMap{"0": 50, "1": 0, "2": 0, "3": 0},
		Memory:     512 * units.MiB,
		NUMAMemory: types.NUMAMemory{},
	})
}

func TestUpdateNodeResourceCapacity(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	node := generateNodes(t, cpuMem, 1, 4, 4*units.GiB, 100)[0]

	resourceOpts := &types.NodeResourceOpts{
		CPUMap:     types.CPUMap{"4": 100, "5": 100},
		Memory:     2 * units.GiB,
		NUMA:       types.NUMA{"0": "0", "1": "0", "2": "0", "3": "1", "4": "1", "5": "1"},
		NUMAMemory: types.NUMAMemory{"0": 3 * units.GiB, "1": 3 * units.GiB},
	}

	// non-existent node
	err := cpuMem.UpdateNodeResourceCapacity(ctx, "xxx", resourceOpts, true)
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	err = cpuMem.UpdateNodeResourceCapacity(ctx, node, resourceOpts, true)
	assert.Nil(t, err)
	resourceInfo, _, err := cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)
	assert.Equal(t, resourceInfo.Capacity, &types.NodeResourceArgs{
		CPU:        6,
		CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100, "4": 100, "5": 100},
		Memory:     6 * units.GiB,
		NUMA:       types.NUMA{"0": "0", "1": "0", "2": "0", "3": "1", "4": "1", "5": "1"},
		NUMAMemory: types.NUMAMemory{"0": 3 * units.GiB, "1": 3 * units.GiB},
	})

	// todo: "decr" doesn't affect numa
	err = cpuMem.UpdateNodeResourceCapacity(ctx, node, resourceOpts, false)
	assert.Nil(t, err)
	resourceInfo, _, err = cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)
	assert.Equal(t, resourceInfo.Capacity, &types.NodeResourceArgs{
		CPU:        4,
		CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
		Memory:     4 * units.GiB,
		NUMA:       types.NUMA{"0": "0", "1": "0", "2": "0", "3": "1", "4": "1", "5": "1"},
		NUMAMemory: types.NUMAMemory{"0": 0, "1": 3 * 0},
	})
}
