package models

import (
	"context"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/cpumem/types"
	coretypes "github.com/projecteru2/core/types"
)

func generateComplexNodes(t *testing.T, cpuMem *CPUMem) []string {
	infos := []*types.NodeResourceInfo{
		{
			Capacity: &types.NodeResourceArgs{
				CPU: 4,
				CPUMap: types.CPUMap{
					"0": 100,
					"1": 100,
					"2": 100,
					"3": 100,
				},
				Memory: 12 * units.GiB,
			},
		},
		{
			Capacity: &types.NodeResourceArgs{
				CPU: 14,
				CPUMap: types.CPUMap{
					"0":  100,
					"1":  100,
					"10": 100,
					"11": 100,
					"12": 100,
					"13": 100,
					"2":  100,
					"3":  100,
					"4":  100,
					"5":  100,
					"6":  100,
					"7":  100,
					"8":  100,
					"9":  100,
				},
				Memory: 12 * units.GiB,
			},
		},
		{
			Capacity: &types.NodeResourceArgs{
				CPU: 12,
				CPUMap: types.CPUMap{
					"0":  100,
					"1":  100,
					"10": 100,
					"11": 100,
					"2":  100,
					"3":  100,
					"4":  100,
					"5":  100,
					"6":  100,
					"7":  100,
					"8":  100,
					"9":  100,
				},
				Memory: 12 * units.GiB,
			},
		},
		{
			Capacity: &types.NodeResourceArgs{
				CPU: 18,
				CPUMap: types.CPUMap{
					"0":  100,
					"1":  100,
					"10": 100,
					"11": 100,
					"12": 100,
					"13": 100,
					"14": 100,
					"15": 100,
					"16": 100,
					"17": 100,
					"2":  100,
					"3":  100,
					"4":  100,
					"5":  100,
					"6":  100,
					"7":  100,
					"8":  100,
					"9":  100,
				},
				Memory: 12 * units.GiB,
			},
		},
		{
			Capacity: &types.NodeResourceArgs{
				CPU: 8,
				CPUMap: types.CPUMap{
					"0": 100,
					"1": 100,
					"2": 100,
					"3": 100,
					"4": 100,
					"5": 100,
					"6": 100,
					"7": 100,
				},
				Memory: 12 * units.GiB,
			},
		},
	}
	nodes := []string{}
	for i, info := range infos {
		nodeName := fmt.Sprintf("node%d", i)
		assert.Nil(t, cpuMem.doSetNodeResourceInfo(context.Background(), nodeName, info))
		nodes = append(nodes, nodeName)
	}
	return nodes
}

func TestGetNodesCapacityWithCPUBinding(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 2, 2, 4*units.GiB, 100)

	// non-existent node
	_, total, err := cpuMem.GetNodesDeployCapacity(ctx, []string{"xxx"}, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 0.5,
		MemRequest: 1,
	})
	assert.True(t, errors.Is(err, coretypes.ErrBadCount))

	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 0.5,
		MemRequest: 1,
	})
	assert.Nil(t, err)
	assert.True(t, total >= 1)

	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 2,
		MemRequest: 1,
	})
	assert.Nil(t, err)
	assert.True(t, total < 3)

	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 3,
		MemRequest: 1,
	})
	assert.Nil(t, err)
	assert.True(t, total < 2)

	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1,
		MemRequest: 1,
	})
	assert.Nil(t, err)
	assert.True(t, total < 5)
}

func TestComplexNodes(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateComplexNodes(t, cpuMem)
	_, total, err := cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.Nil(t, err)
	assert.Equal(t, 28, total)
}

func TestCPUNodesWithMemoryLimit(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 2, 2, 1024, 100)
	_, total, err := cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 0.1,
		MemRequest: 1024,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, 2)

	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 0.1,
		MemRequest: 1025,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, 0)
}

func TestCPUNodesWithMaxShareLimit(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	cpuMem.Config.Scheduler.MaxShare = 2

	nodes := generateNodes(t, cpuMem, 1, 6, 12*units.GiB, 100)
	_, total, err := cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, 2)

	nodeResourceInfo := &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    4,
		CPUMap: types.CPUMap{"0": 0, "1": 0, "2": 100, "3": 100},
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, nodeResourceInfo.Validate())
	_, _, err = cpuMem.doAllocByCPU(nodeResourceInfo, 1, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.2,
		MemRequest: 1,
	})
	assert.Nil(t, err)
}

func BenchmarkGetNodesCapacity(b *testing.B) {
	b.StopTimer()
	t := &testing.T{}
	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 10000, 24, 128*units.GiB, 100)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := cpuMem.GetNodesDeployCapacity(context.Background(), nodes, &types.WorkloadResourceOpts{
			CPUBind:    true,
			CPURequest: 1.3,
			MemRequest: 1,
		})
		assert.Nil(b, err)
	}
}

func TestGetNodesCapacityByMemory(t *testing.T) {
	ctx := context.Background()

	cpuMem := newTestCPUMem(t)
	nodes := generateNodes(t, cpuMem, 2, 2, 4*units.GiB, 100)

	// negative memory
	_, _, err := cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    false,
		CPURequest: 0,
		MemRequest: -1,
	})
	assert.True(t, errors.Is(err, types.ErrInvalidMemory))

	// cpu + mem
	_, total, err := cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    false,
		CPURequest: 1,
		MemRequest: 512 * units.MiB,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, 16)

	// unlimited cpu
	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    false,
		CPURequest: 0,
		MemRequest: 512 * units.MiB,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, 16)

	// insufficient cpu
	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    false,
		CPURequest: 3,
		MemRequest: 512 * units.MiB,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, 0)

	// mem_request == 0
	_, total, err = cpuMem.GetNodesDeployCapacity(ctx, nodes, &types.WorkloadResourceOpts{
		CPUBind:    false,
		CPURequest: 1,
		MemRequest: 0,
	})
	assert.Nil(t, err)
	assert.Equal(t, total, math.MaxInt)
}
