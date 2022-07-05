package schedule

import (
	"strconv"
	"testing"

	"github.com/projecteru2/core/resources/cpumem/types"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

func TestGetFullCPUPlans(t *testing.T) {
	h := newHost(types.CPUMap{
		"0": 400,
		"1": 200,
		"2": 400,
	}, 100, -1)
	cpuPlans := h.getFullCPUPlans(h.fullCores, 2)
	assert.Equal(t, 5, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []types.CPUMap{
		{"0": 100, "1": 100},
		{"0": 100, "2": 100},
		{"0": 100, "2": 100},
		{"0": 100, "2": 100},
		{"1": 100, "2": 100},
	})

	h = newHost(types.CPUMap{
		"0": 200,
		"1": 200,
		"2": 200,
	}, 100, -1)
	cpuPlans = h.getFullCPUPlans(h.fullCores, 2)
	assert.EqualValues(t, 3, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []types.CPUMap{
		{"0": 100, "1": 100},
		{"0": 100, "2": 100},
		{"1": 100, "2": 100},
	})
}

func TestGetCPUPlansWithAffinity(t *testing.T) {
	// 1.7 -> 1.0
	cpuMap := types.CPUMap{
		"0": 0,
		"1": 30,
		"2": 0,
	}
	originCPUMap := types.CPUMap{
		"0": 100,
		"1": 30,
		"2": 40,
	}
	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans := GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 1})
	assert.Equal(t, 1, len(cpuPlans))
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 100})

	// 1.7 -> 1.2
	cpuMap = types.CPUMap{
		"0": 0,
		"1": 30,
		"2": 0,
	}
	originCPUMap = types.CPUMap{
		"0": 100,
		"1": 30,
		"2": 40,
	}

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans = GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 1.2})
	assert.Equal(t, 1, len(cpuPlans))
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 100, "1": 20})

	// 1.7 -> 2
	cpuMap = types.CPUMap{
		"0": 0,
		"1": 80,
		"2": 0,
		"3": 0,
	}
	originCPUMap = types.CPUMap{
		"0": 100,
		"1": 20,
		"2": 40,
		"3": 10,
	}

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans = GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 2})
	assert.Equal(t, 1, len(cpuPlans))
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 100, "1": 100})

	// 1.7 -> 2 without enough pieces
	cpuMap = types.CPUMap{
		"0": 0,
		"1": 69,
		"2": 10,
	}
	originCPUMap = types.CPUMap{
		"0": 100,
		"1": 30,
		"2": 40,
	}

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans = GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 2})
	assert.Equal(t, 0, len(cpuPlans))

	// 1.7 -> 2
	cpuMap = types.CPUMap{
		"0": 0,
		"1": 70,
		"2": 10,
	}
	originCPUMap = types.CPUMap{
		"0": 100,
		"1": 30,
		"2": 40,
	}

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans = GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 2})
	assert.Equal(t, 1, len(cpuPlans))
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 100, "1": 100})

	// 1.7 -> 2
	cpuMap = types.CPUMap{
		"0": 100,
		"1": 60,
		"2": 0,
		"3": 100,
		"4": 100,
	}
	originCPUMap = types.CPUMap{
		"0": 100,
		"1": 30,
		"2": 40,
	}

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans = GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 2})
	assert.Equal(t, 2, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 100, "3": 100}},
		{CPUMap: types.CPUMap{"0": 100, "4": 100}},
	})

	// 1.7 -> 2
	cpuMap = types.CPUMap{
		"0": 0,
		"1": 60,
		"2": 0,
	}
	originCPUMap = types.CPUMap{
		"0": 100,
		"1": 30,
		"2": 40,
	}

	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{CPUMap: cpuMap, CPU: float64(len(cpuMap))},
		Usage:    &types.NodeResourceArgs{},
	}
	resourceInfo.Capacity.CPUMap.Add(originCPUMap)
	cpuPlans = GetCPUPlans(resourceInfo, originCPUMap, 100, -1, &types.WorkloadResourceOpts{CPUBind: true, CPURequest: 2})
	assert.Equal(t, 0, len(cpuPlans))
}

func TestCPUOverSell(t *testing.T) {
	var cpuMap types.CPUMap
	var resourceInfo *types.NodeResourceInfo
	maxShare := -1
	shareBase := 100

	// oversell
	cpuMap = types.CPUMap{"0": 300, "1": 300}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 2,
		MemRequest: 1,
	})
	assert.Equal(t, len(cpuPlans), 3)
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 100, "1": 100}},
		{CPUMap: types.CPUMap{"0": 100, "1": 100}},
		{CPUMap: types.CPUMap{"0": 100, "1": 100}},
	})

	// one core oversell
	cpuMap = types.CPUMap{"0": 300}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 0.5,
		MemRequest: 1,
	})
	assert.Equal(t, len(cpuPlans), 6)
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 50}},
		{CPUMap: types.CPUMap{"0": 50}},
		{CPUMap: types.CPUMap{"0": 50}},
		{CPUMap: types.CPUMap{"0": 50}},
		{CPUMap: types.CPUMap{"0": 50}},
		{CPUMap: types.CPUMap{"0": 50}},
	})

	// balance
	cpuMap = types.CPUMap{"0": 100, "1": 200, "2": 300}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1,
		MemRequest: 1,
	})
	assert.Equal(t, len(cpuPlans), 6)
	assert.ElementsMatch(t, cpuPlans[:2], []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 100}},
		{CPUMap: types.CPUMap{"1": 100}},
	})

	// complex
	cpuMap = types.CPUMap{"0": 50, "1": 100, "2": 300, "3": 70, "4": 200, "5": 30, "6": 230}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) >= 2)

	cpuMap = types.CPUMap{"0": 70, "1": 100, "2": 400}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.3,
		MemRequest: 1,
	})
	assert.Equal(t, len(cpuPlans), 4)
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 30, "2": 100}},
		{CPUMap: types.CPUMap{"0": 30, "2": 100}},
		{CPUMap: types.CPUMap{"1": 30, "2": 100}},
		{CPUMap: types.CPUMap{"1": 30, "2": 100}},
	})
}

func applyCPUPlans(t *testing.T, resourceInfo *types.NodeResourceInfo, cpuPlans []*CPUPlan) {
	for _, cpuPlan := range cpuPlans {
		resourceInfo.Usage.CPUMap.Add(cpuPlan.CPUMap)
	}
	assert.Nil(t, resourceInfo.Validate())
}

func TestCPUOverSellAndStableFragmentCore(t *testing.T) {
	var cpuMap types.CPUMap
	var resourceInfo *types.NodeResourceInfo
	var cpuPlans []*CPUPlan
	maxShare := -1
	shareBase := 100

	// oversell
	cpuMap = types.CPUMap{"0": 300, "1": 300}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) > 0)

	// stable fragment core
	cpuMap = types.CPUMap{"0": 230, "1": 200}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) > 0)
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 70, "1": 100})
	applyCPUPlans(t, resourceInfo, cpuPlans[:1])

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) > 0)
	assert.Equal(t, cpuPlans[0].CPUMap, types.CPUMap{"0": 70, "1": 100})

	// complex node
	cpuMap = types.CPUMap{"0": 230, "1": 80, "2": 300, "3": 200}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) >= 2)
	applyCPUPlans(t, resourceInfo, cpuPlans[:2])
	assert.Equal(t, resourceInfo.Usage.CPUMap, types.CPUMap{"0": 70, "1": 70, "2": 0, "3": 200})

	// consume full core
	cpuMap = types.CPUMap{"0": 70, "1": 50, "2": 100, "3": 100, "4": 100}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.7,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) >= 2)
	applyCPUPlans(t, resourceInfo, cpuPlans[:2])
	assert.Equal(t, resourceInfo.Usage.CPUMap, types.CPUMap{"0": 70, "1": 0, "2": 70, "3": 100, "4": 100})

	// consume less fragment core
	cpuMap = types.CPUMap{"0": 70, "1": 50, "2": 90}
	resourceInfo = &types.NodeResourceInfo{Capacity: &types.NodeResourceArgs{
		CPU:    float64(len(cpuMap)),
		CPUMap: cpuMap,
		Memory: 12 * units.GiB,
	}}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 0.5,
		MemRequest: 1,
	})
	assert.True(t, len(cpuPlans) >= 2)
	applyCPUPlans(t, resourceInfo, cpuPlans[:2])
	assert.Equal(t, resourceInfo.Usage.CPUMap, types.CPUMap{"0": 50, "1": 50, "2": 0})
}

func TestNUMANodes(t *testing.T) {
	maxShare := -1
	shareBase := 100

	// same numa node
	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:        4,
			CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Memory:     4 * units.GiB,
			NUMAMemory: types.NUMAMemory{"0": 2 * units.GiB, "1": 2 * units.GiB},
			NUMA:       types.NUMA{"0": "0", "1": "0", "2": "1", "3": "1"},
		},
		Usage: nil,
	}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.3,
		MemRequest: 1,
	})
	assert.Equal(t, 2, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 30, "1": 100}, NUMANode: "0"},
		{CPUMap: types.CPUMap{"2": 30, "3": 100}, NUMANode: "1"},
	})

	// same numa node + cross numa node
	resourceInfo = &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:        4,
			CPUMap:     types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100, "4": 100, "5": 100},
			Memory:     6 * units.GiB,
			NUMAMemory: types.NUMAMemory{"0": 3 * units.GiB, "1": 3 * units.GiB},
			NUMA:       types.NUMA{"0": "0", "1": "0", "2": "0", "3": "1", "4": "1", "5": "1"},
		},
		Usage: nil,
	}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans = GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 2,
		MemRequest: 2 * units.GiB,
	})
	assert.Equal(t, 3, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"1": 100, "2": 100}, NUMANode: "0"},
		{CPUMap: types.CPUMap{"4": 100, "5": 100}, NUMANode: "1"},
		{CPUMap: types.CPUMap{"0": 100, "3": 100}, NUMANode: ""},
	})
}

func TestInsufficientMemory(t *testing.T) {
	maxShare := -1
	shareBase := 100

	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:    4,
			CPUMap: types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Memory: 4 * units.GiB,
		},
		Usage: nil,
	}
	assert.Nil(t, resourceInfo.Validate())

	cpuPlans := GetCPUPlans(resourceInfo, nil, shareBase, maxShare, &types.WorkloadResourceOpts{
		CPUBind:    true,
		CPURequest: 1.3,
		MemRequest: 3 * units.GiB,
	})
	assert.Equal(t, 1, len(cpuPlans))
	assert.ElementsMatch(t, cpuPlans, []*CPUPlan{
		{CPUMap: types.CPUMap{"0": 30, "1": 100}},
	})
}

func BenchmarkGetCPUPlans(b *testing.B) {
	b.StopTimer()
	resourceInfo := &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			CPU:    24,
			CPUMap: types.CPUMap{},
			Memory: 128 * units.GiB,
		},
	}
	for i := 0; i < 24; i++ {
		resourceInfo.Capacity.CPUMap[strconv.Itoa(i)] = 100
	}
	assert.Nil(b, resourceInfo.Validate())
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		assert.True(b, len(GetCPUPlans(resourceInfo, nil, 100, -1, &types.WorkloadResourceOpts{
			CPUBind:    true,
			CPURequest: 1.3,
			MemRequest: 1,
		})) > 0)
	}
}
