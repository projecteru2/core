package models

import (
	"context"
	"errors"
	"testing"

	"github.com/projecteru2/core/resources/cpumem/types"
	coretypes "github.com/projecteru2/core/types"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

func TestGetNodeResourceInfo(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 2, 4*units.GiB, 100)
	node := nodes[0]

	// invalid node
	_, _, err := cpuMem.GetNodeResourceInfo(ctx, "xxx", nil, false)
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	resourceInfo, diffs, err := cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(diffs))

	resourceInfo.Capacity.NUMA = types.NUMA{"0": "0", "1": "1"}
	resourceInfo.Capacity.NUMAMemory = types.NUMAMemory{"0": 2 * units.GiB, "1": 2 * units.GiB}

	assert.Nil(t, cpuMem.SetNodeResourceInfo(ctx, node, resourceInfo.Capacity, resourceInfo.Usage))

	resourceInfo, diffs, err = cpuMem.GetNodeResourceInfo(ctx, node, &types.WorkloadResourceArgsMap{
		"x-workload": {
			CPURequest:    2,
			CPUMap:        types.CPUMap{"0": 100, "1": 100},
			MemoryRequest: 2 * units.GiB,
			NUMAMemory:    types.NUMAMemory{"0": units.GiB, "1": units.GiB},
		},
	}, true)
	assert.Nil(t, err)
	assert.Equal(t, 6, len(diffs))
	assert.Equal(t, resourceInfo.Usage, &types.NodeResourceArgs{
		CPU:        2,
		CPUMap:     types.CPUMap{"0": 100, "1": 100},
		Memory:     2 * units.GiB,
		NUMAMemory: types.NUMAMemory{"0": units.GiB, "1": units.GiB},
		NUMA:       types.NUMA{},
	})
}

func TestSetNodeResourceInfo(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 2, 4*units.GiB, 100)
	node := nodes[0]

	resourceInfo, _, err := cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)

	err = cpuMem.SetNodeResourceInfo(ctx, "node-x", resourceInfo.Capacity, resourceInfo.Usage)
	assert.Nil(t, err)
}

func TestSetNodeResourceUsage(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 2, 4*units.GiB, 100)
	node := nodes[0]

	_, _, err := cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)

	var after *types.NodeResourceArgs

	nodeResourceOpts := &types.NodeResourceOpts{
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		Memory: 2 * units.GiB,
	}

	nodeResourceArgs := &types.NodeResourceArgs{
		CPU: 2,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		Memory: 2 * units.GiB,
	}

	workloadResourceArgs := []*types.WorkloadResourceArgs{{
		CPURequest: 2,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		MemoryRequest: 2 * units.GiB,
	},
	}

	originResourceUsage := &types.NodeResourceArgs{
		CPU: 0,
		CPUMap: map[string]int{
			"0": 0,
			"1": 0,
		},
		Memory:     0,
		NUMAMemory: types.NUMAMemory{},
		NUMA:       types.NUMA{},
	}

	afterSetNodeResourceUsageDelta := &types.NodeResourceArgs{
		CPU: 2,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		Memory:     2 * units.GiB,
		NUMAMemory: types.NUMAMemory{},
		NUMA:       types.NUMA{},
	}

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nodeResourceOpts, nil, nil, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nodeResourceOpts, nil, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nil, nodeResourceArgs, nil, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nil, nodeResourceArgs, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nil, nil, workloadResourceArgs, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDelta)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nil, nil, workloadResourceArgs, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nil, nil, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, originResourceUsage, after)

	_, after, err = cpuMem.SetNodeResourceUsage(ctx, node, nil, nodeResourceArgs, nil, false, true)
	assert.Nil(t, err)
	assert.Equal(t, nodeResourceArgs.DeepCopy(), after.DeepCopy())
}

func TestSetNodeResourceCapacity(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 2, 2*units.GiB, 100)
	node := nodes[0]

	_, _, err := cpuMem.GetNodeResourceInfo(ctx, node, nil, false)
	assert.Nil(t, err)

	var after *types.NodeResourceArgs

	nodeResourceOptsDelta := &types.NodeResourceOpts{
		CPUMap: map[string]int{
			"2": 100,
			"3": 100,
		},
		Memory: 0,
		NUMAMemory: types.NUMAMemory{
			"0": units.GiB,
			"1": units.GiB,
		},
		NUMA: types.NUMA{
			"0": "0",
			"1": "0",
			"2": "1",
			"3": "1",
		},
	}

	nodeResourceArgsDelta := &types.NodeResourceArgs{
		CPU: 2,
		CPUMap: map[string]int{
			"2": 100,
			"3": 100,
		},
		Memory: 0,
		NUMAMemory: types.NUMAMemory{
			"0": units.GiB,
			"1": units.GiB,
		},
		NUMA: types.NUMA{
			"0": "0",
			"1": "0",
			"2": "1",
			"3": "1",
		},
	}

	originResourceCapacity := &types.NodeResourceArgs{
		CPU: 2,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		Memory:     2 * units.GiB,
		NUMAMemory: types.NUMAMemory{},
		NUMA:       types.NUMA{},
	}

	originResourceCapacityWithNUMAChanged := &types.NodeResourceArgs{
		CPU: 2,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		Memory: 2 * units.GiB,
		NUMAMemory: types.NUMAMemory{
			"0": 0,
			"1": 0,
		},
		NUMA: types.NUMA{
			"0": "0",
			"1": "0",
			"2": "1",
			"3": "1",
		},
	}

	originResourceCapacityOpts := &types.NodeResourceOpts{
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
		},
		Memory:     2 * units.GiB,
		NUMAMemory: types.NUMAMemory{},
		NUMA:       types.NUMA{},
		RawParams: coretypes.RawParams{
			"cpu": "",
		},
	}

	afterSetNodeResourceUsageDeltaWithNUMAChanged := &types.NodeResourceArgs{
		CPU: 4,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
			"2": 100,
			"3": 100,
		},
		Memory: 2 * units.GiB,
		NUMAMemory: types.NUMAMemory{
			"0": units.GiB,
			"1": units.GiB,
		},
		NUMA: types.NUMA{
			"0": "0",
			"1": "0",
			"2": "1",
			"3": "1",
		},
	}

	afterSetNodeResourceUsageDeltaWithNUMANotChanged := &types.NodeResourceArgs{
		CPU: 4,
		CPUMap: map[string]int{
			"0": 100,
			"1": 100,
			"2": 100,
			"3": 100,
		},
		Memory:     2 * units.GiB,
		NUMAMemory: types.NUMAMemory{},
		NUMA:       types.NUMA{},
	}

	_, after, err = cpuMem.SetNodeResourceCapacity(ctx, node, nodeResourceOptsDelta, nil, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDeltaWithNUMAChanged)

	_, after, err = cpuMem.SetNodeResourceCapacity(ctx, node, nodeResourceOptsDelta, nil, true, false)
	assert.Nil(t, err)
	assert.Equal(t, after, originResourceCapacityWithNUMAChanged)

	_, after, err = cpuMem.SetNodeResourceCapacity(ctx, node, nil, nodeResourceArgsDelta, true, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDeltaWithNUMAChanged)

	_, after, err = cpuMem.SetNodeResourceCapacity(ctx, node, nil, nodeResourceArgsDelta, true, false)
	assert.Nil(t, err)
	assert.Equal(t, after, originResourceCapacityWithNUMAChanged)

	_, after, err = cpuMem.SetNodeResourceCapacity(ctx, node, nil, afterSetNodeResourceUsageDeltaWithNUMANotChanged, false, true)
	assert.Nil(t, err)
	assert.Equal(t, after, afterSetNodeResourceUsageDeltaWithNUMANotChanged)

	_, after, err = cpuMem.SetNodeResourceCapacity(ctx, node, originResourceCapacityOpts, nil, false, true)
	assert.Nil(t, err)
	assert.Equal(t, after, originResourceCapacity)
}
