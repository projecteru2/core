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

func TestRealloc(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	node := "node"
	resourceOpts := &types.NodeResourceOpts{
		CPUMap: types.CPUMap{"0": 100, "1": 100},
		Memory: 4 * units.GiB,
		NUMA:   types.NUMA{"0": "0", "1": "1"},
	}

	_, err := cpuMem.AddNode(ctx, node, resourceOpts)
	assert.Nil(t, err)

	originResourceArgs := &types.WorkloadResourceArgs{
		CPURequest:    1,
		CPULimit:      1,
		MemoryRequest: units.GiB,
		MemoryLimit:   units.GiB,
		CPUMap:        types.CPUMap{"0": 100},
		NUMAMemory:    types.NUMAMemory{"0": units.GiB},
		NUMANode:      "0",
	}

	// non-existent node
	_, _, _, err = cpuMem.GetReallocArgs(ctx, "xxx", originResourceArgs, &types.WorkloadResourceOpts{})
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	// invalid resource opts
	opts := &types.WorkloadResourceOpts{
		CPUBind:     false,
		KeepCPUBind: true,
		CPURequest:  -3,
		CPULimit:    0,
		MemRequest:  0,
		MemLimit:    0,
	}
	_, _, _, err = cpuMem.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.True(t, errors.Is(err, types.ErrInvalidCPU))

	// insufficient cpu
	opts = &types.WorkloadResourceOpts{
		CPUBind:     false,
		KeepCPUBind: true,
		CPURequest:  2,
		CPULimit:    0,
		MemRequest:  0,
		MemLimit:    0,
	}
	_, _, _, err = cpuMem.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.True(t, errors.Is(err, types.ErrInsufficientResource))

	// normal case (with cpu-bind)
	opts = &types.WorkloadResourceOpts{
		CPUBind:     false,
		KeepCPUBind: true,
		CPURequest:  -0.5,
		CPULimit:    -0.5,
		MemRequest:  0,
		MemLimit:    0,
	}
	_, _, _, err = cpuMem.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.Nil(t, err)

	// normal case (without cpu-bind)
	opts = &types.WorkloadResourceOpts{
		CPUBind:     false,
		KeepCPUBind: false,
		CPURequest:  0,
		CPULimit:    0,
		MemRequest:  0,
		MemLimit:    0,
	}
	_, _, _, err = cpuMem.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.Nil(t, err)

	// insufficient mem
	opts = &types.WorkloadResourceOpts{
		CPUBind:     false,
		KeepCPUBind: false,
		CPURequest:  0,
		CPULimit:    0,
		MemRequest:  units.PiB,
		MemLimit:    units.PiB,
	}
	_, _, _, err = cpuMem.GetReallocArgs(ctx, node, originResourceArgs, opts)
	assert.True(t, errors.Is(err, types.ErrInsufficientMem))
}
