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

func TestAlloc(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 1, 2, 4*units.GiB, 100)
	node := nodes[0]

	// invalid opts
	_, _, err := cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: -1,
	})
	assert.True(t, errors.Is(err, types.ErrInvalidCPU))

	// non-existent node
	_, _, err = cpuMem.GetDeployArgs(ctx, "xxx", 1, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1,
	})
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	// cpu bind
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.1,
	})
	assert.Nil(t, err)

	// cpu bind & insufficient resource
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 2.2,
	})
	assert.True(t, errors.Is(err, types.ErrInsufficientResource))
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 3, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1,
	})
	assert.True(t, errors.Is(err, types.ErrInsufficientResource))

	// alloc by memory
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		MemRequest: units.GiB,
	})
	assert.Nil(t, err)

	// alloc by memory & insufficient cpu
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		MemRequest: units.GiB,
		CPURequest: 65535,
	})
	assert.True(t, errors.Is(err, types.ErrInsufficientCPU))

	// alloc by memory & insufficient mem
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		MemRequest: 5 * units.GiB,
		CPURequest: 1,
	})
	assert.True(t, errors.Is(err, types.ErrInsufficientMem))

	// mem_request == 0
	_, _, err = cpuMem.GetDeployArgs(ctx, node, 1, &types.WorkloadResourceOpts{
		MemRequest: 0,
		CPURequest: 1,
	})
	assert.Nil(t, err)

	// numa node
	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:        4,
			CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Memory:     4 * units.GiB,
			NUMAMemory: types.NUMAMemory{"0": 2 * units.GiB, "1": 2 * units.GiB},
			NUMA:       types.NUMA{"0": "0", "1": "0", "2": "1", "3": "1"},
		},
	}
	assert.Nil(t, resourceInfo.Validate())
	assert.Nil(t, cpuMem.doSetNodeResourceInfo(ctx, "numa-node", resourceInfo))
	_, resourceArgs, err := cpuMem.GetDeployArgs(ctx, "numa-node", 2, &types.WorkloadResourceOpts{
		CPUBind:    true,
		MemRequest: 1 * units.GiB,
		CPURequest: 1.3,
	})
	assert.Nil(t, err)
	assert.ElementsMatch(t, []string{resourceArgs[0].NUMANode, resourceArgs[1].NUMANode}, []string{"0", "1"})
}
