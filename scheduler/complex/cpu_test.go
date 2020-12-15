package complexscheduler

import (
	"reflect"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
)

func TestCPUPriorPlan(t *testing.T) {
	// normal 分配
	scheduleInfos := resetscheduleInfos()
	_, resultCPUPlan, total, err := cpuPriorPlan(3.0, int64(units.MiB), scheduleInfos, -1, 100)
	assert.NoError(t, err)
	assert.Equal(t, len(resultCPUPlan), 1)
	assert.Equal(t, total, 1)
	// numa 分配
	scheduleInfos = resetscheduleInfos()
	_, resultCPUPlan, total, err = cpuPriorPlan(1.5, int64(units.MiB), scheduleInfos, -1, 100)
	assert.NoError(t, err)
	assert.Equal(t, len(resultCPUPlan), 1)
	assert.Equal(t, total, 2)
	r := resultCPUPlan["n1"]
	for _, p := range r {
		_, ok1 := p["1"]
		_, ok2 := p["2"]
		_, ok3 := p["3"]
		_, ok4 := p["4"]
		assert.True(t, (ok1 && ok3) || (ok2 && ok4))
	}
	// numa and normal 分配
	scheduleInfos = resetscheduleInfos()
	_, resultCPUPlan, total, err = cpuPriorPlan(1, int64(units.GiB), scheduleInfos, -1, 100)
	assert.NoError(t, err)
	assert.Equal(t, len(resultCPUPlan), 1)
	assert.Equal(t, total, 3)
}

func resetscheduleInfos() []resourcetypes.ScheduleInfo {
	return []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name:   "n1",
				CPU:    types.CPUMap{"1": 100, "2": 100, "3": 100, "4": 100},
				MemCap: 3 * int64(units.GiB),
				NUMA: types.NUMA{
					"1": "node0",
					"2": "node1",
					"3": "node0",
					"4": "node1",
				},
				NUMAMemory: types.NUMAMemory{
					"node0": int64(units.GiB),
					"node1": int64(units.GiB),
				},
			},
		},
	}
}

func TestCPUReallocPlan(t *testing.T) {
	// scheduleinfo not found
	scheduleInfos := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
			},
		},
	}
	existCPUInfo := map[string]types.CPUMap{
		"n2": {
			"0": 100,
		},
	}
	_, remain, aff := cpuReallocPlan(scheduleInfos, 1, existCPUInfo, 100)
	assert.Nil(t, aff)
	assert.EqualValues(t, 1, remain)

	// shrink: 1.7->1.0
	scheduleInfos = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 0,
					"1": 30,
					"2": 0,
				},
			},
		},
	}
	existCPUInfo = map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 30,
			"2": 40,
		},
	}
	sis, remain, aff := cpuReallocPlan(scheduleInfos, 1, existCPUInfo, 100)
	assert.EqualValues(t, 0, remain)
	assert.True(t, reflect.DeepEqual(aff, types.CPUMap{"0": 100}))
	assert.True(t, reflect.DeepEqual(sis[0].CPU, types.CPUMap{"0": 0, "1": 60, "2": 40}))

	// shrink: 1.7->1.2
	scheduleInfos = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 0,
					"1": 30,
					"2": 0,
				},
			},
		},
	}
	existCPUInfo = map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 30,
			"2": 40,
		},
	}
	sis, remain, aff = cpuReallocPlan(scheduleInfos, 1.2, existCPUInfo, 100)
	assert.EqualValues(t, 0, remain)
	assert.True(t, reflect.DeepEqual(aff, types.CPUMap{"0": 100, "2": 20}))
	assert.True(t, reflect.DeepEqual(sis[0].CPU, types.CPUMap{"0": 0, "1": 60, "2": 20}))

	// expand: 1.7->2, find complement
	scheduleInfos = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 0,
					"1": 80,
					"2": 0,
					"3": 0,
				},
			},
		},
	}
	existCPUInfo = map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 20,
			"2": 40,
			"3": 10,
		},
	}
	sis, remain, aff = cpuReallocPlan(scheduleInfos, 2, existCPUInfo, 100)
	assert.EqualValues(t, 0, remain)
	assert.True(t, reflect.DeepEqual(aff, types.CPUMap{"0": 100, "1": 100}))
	assert.True(t, reflect.DeepEqual(sis[0].CPU, types.CPUMap{"0": 0, "1": 0, "2": 40, "3": 10}))

	// expand: 1.7->2, lose complement
	scheduleInfos = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 0,
					"1": 69,
					"2": 10,
				},
			},
		},
	}
	existCPUInfo = map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 30,
			"2": 40,
		},
	}
	sis, remain, aff = cpuReallocPlan(scheduleInfos, 2, existCPUInfo, 100)
	assert.EqualValues(t, 1, remain)
	assert.True(t, reflect.DeepEqual(aff, types.CPUMap{"0": 100}))
	assert.True(t, reflect.DeepEqual(sis[0].CPU, types.CPUMap{"0": 0, "1": 99, "2": 50}))
}

func TestCPUReallocWithPriorPlan(t *testing.T) {
	po, err := New(types.Config{Scheduler: types.SchedConfig{
		MaxShare:  0,
		ShareBase: 100,
	}})
	assert.Nil(t, err)

	// direct return after realloc plan
	scheduleInfos := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 0,
					"1": 70,
					"2": 0,
				},
			},
		},
	}
	existCPUInfo := map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 30,
			"2": 40,
		},
	}
	rsi, cpuPlans, total, err := po.SelectCPUNodes(scheduleInfos, 2, 0, existCPUInfo)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, total)
	assert.True(t, reflect.DeepEqual(cpuPlans, map[string][]types.CPUMap{"n1": {{"0": 100, "1": 100}}}))
	assert.EqualValues(t, 1, rsi[0].Capacity)

	// realloc plan + cpu prior plan
	scheduleInfos = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 100,
					"1": 60,
					"2": 0,
					"3": 100,
					"4": 100,
				},
			},
		},
	}
	existCPUInfo = map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 30,
			"2": 40,
		},
	}
	rsi, cpuPlans, total, err = po.SelectCPUNodes(scheduleInfos, 2, 0, existCPUInfo)
	assert.Nil(t, err)
	assert.EqualValues(t, 3, total)
	asserted := 0
	for _, plan := range cpuPlans["n1"] {
		if _, ok := plan["3"]; ok {
			assert.True(t, reflect.DeepEqual(plan, types.CPUMap{"0": 100, "3": 100}))
			asserted++
		} else if _, ok := plan["4"]; ok {
			assert.True(t, reflect.DeepEqual(plan, types.CPUMap{"0": 100, "4": 100}))
			asserted++
		} else {
			assert.True(t, reflect.DeepEqual(plan, types.CPUMap{"0": 200}))
			asserted++
		}
	}
	assert.EqualValues(t, 3, asserted)
	assert.EqualValues(t, 3, rsi[0].Capacity)

	// realloc plan + cpu prior error
	scheduleInfos = []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name: "n1",
				CPU: types.CPUMap{
					"0": 0,
					"1": 60,
					"2": 0,
				},
			},
		},
	}
	existCPUInfo = map[string]types.CPUMap{
		"n1": {
			"0": 100,
			"1": 30,
			"2": 40,
		},
	}
	rsi, cpuPlans, total, err = po.SelectCPUNodes(scheduleInfos, 2, 0, existCPUInfo)
	assert.EqualError(t, err, "not enough resource")
}
