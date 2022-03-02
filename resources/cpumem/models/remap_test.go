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

func TestRemap(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	node := "node"
	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:    4,
			CPUMap: types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Memory: 4 * units.GiB,
		},
	}
	assert.Nil(t, cpuMem.doSetNodeResourceInfo(ctx, node, resourceInfo))

	// non-existent node
	_, err := cpuMem.GetRemapArgs(ctx, "xxx", nil)
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	// normal case
	workloadResourceMap := &types.WorkloadResourceArgsMap{
		"w1": {
			CPUMap:        nil,
			CPURequest:    1,
			CPULimit:      1,
			MemoryRequest: units.GiB,
			MemoryLimit:   units.GiB,
		},
		"w2": {
			CPUMap:        nil,
			CPURequest:    1,
			CPULimit:      1,
			MemoryRequest: units.GiB,
			MemoryLimit:   units.GiB,
		},
		"w3": {
			CPUMap:        types.CPUMap{"0": 100, "1": 100},
			CPURequest:    2,
			CPULimit:      2,
			MemoryRequest: units.GiB,
			MemoryLimit:   units.GiB,
		},
	}
	resourceInfo, _, err = cpuMem.GetNodeResourceInfo(ctx, node, workloadResourceMap, true)
	assert.Nil(t, err)

	engineArgsMap, err := cpuMem.GetRemapArgs(ctx, node, workloadResourceMap)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(engineArgsMap))
	assert.Equal(t, engineArgsMap["w1"], &types.EngineArgs{
		CPU:      1,
		CPUMap:   types.CPUMap{"2": 100, "3": 100},
		NUMANode: "",
		Memory:   units.GiB,
		Remap:    true,
	})
	assert.Equal(t, engineArgsMap["w2"], &types.EngineArgs{
		CPU:      1,
		CPUMap:   types.CPUMap{"2": 100, "3": 100},
		NUMANode: "",
		Memory:   units.GiB,
		Remap:    true,
	})

	// empty share cpu map
	(*workloadResourceMap)["w4"] = &types.WorkloadResourceArgs{
		CPUMap:        types.CPUMap{"2": 100, "3": 100},
		CPURequest:    2,
		CPULimit:      2,
		MemoryRequest: units.GiB,
		MemoryLimit:   units.GiB,
	}

	_, _, err = cpuMem.GetNodeResourceInfo(ctx, node, workloadResourceMap, true)
	assert.Nil(t, err)

	engineArgsMap, err = cpuMem.GetRemapArgs(ctx, node, workloadResourceMap)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(engineArgsMap))
	assert.Equal(t, engineArgsMap["w1"], &types.EngineArgs{
		CPU:      1,
		CPUMap:   types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
		NUMANode: "",
		Memory:   units.GiB,
		Remap:    true,
	})
	assert.Equal(t, engineArgsMap["w2"], &types.EngineArgs{
		CPU:      1,
		CPUMap:   types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
		NUMANode: "",
		Memory:   units.GiB,
		Remap:    true,
	})
}
