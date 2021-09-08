package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCPUMap(t *testing.T) {
	cpuMap := CPUMap{"0": 50, "1": 70}
	total := cpuMap.Total()
	assert.Equal(t, total, int64(120))

	cpuMap.Add(CPUMap{"0": 20})
	assert.Equal(t, cpuMap["0"], int64(70))

	cpuMap.Add(CPUMap{"3": 100})
	assert.Equal(t, cpuMap["3"], int64(100))

	cpuMap.Sub(CPUMap{"1": 20})
	assert.Equal(t, cpuMap["1"], int64(50))
}

func TestNewVolumePlan(t *testing.T) {
	plan := MakeVolumePlan(
		MustToVolumeBindings([]string{"AUTO:/data0:rw:10", "AUTO:/data1:ro:20", "AUTO:/data2:rw:10"}),
		[]VolumeMap{
			{"/dir0": 10},
			{"/dir1": 10},
			{"/dir2": 20},
		},
	)
	assert.Equal(t, plan, VolumePlan{
		MustToVolumeBinding("AUTO:/data0:rw:10"): VolumeMap{"/dir0": 10},
		MustToVolumeBinding("AUTO:/data1:ro:20"): VolumeMap{"/dir2": 20},
		MustToVolumeBinding("AUTO:/data2:rw:10"): VolumeMap{"/dir1": 10},
	})

	data := []byte(`{"AUTO:/data0:rw:10":{"/dir0":10},"AUTO:/data1:ro:20":{"/dir2":20},"AUTO:/data2:rw:10":{"/dir1":10}}`)
	b, err := json.Marshal(plan)
	assert.Nil(t, err)
	assert.Equal(t, b, data)

	plan2 := VolumePlan{}
	err = json.Unmarshal(data, &plan2)
	assert.Nil(t, err)
	assert.Equal(t, plan2, plan)

	plan3 := MakeVolumePlan(
		MustToVolumeBindings([]string{"AUTO:/data3:rw:10"}),
		[]VolumeMap{
			{"/dir0": 10},
		},
	)
	plan.Merge(plan3)
	assert.Equal(t, len(plan), 4)
	assert.Equal(t, plan[MustToVolumeBinding("AUTO:/data3:rw:10")], VolumeMap{"/dir0": 10})
}

func TestVolumeMap(t *testing.T) {
	volume := VolumeMap{"/data": 1000}
	assert.Equal(t, volume.Total(), int64(1000))
	assert.Equal(t, volume.GetResourceID(), "/data")
	assert.Equal(t, volume.GetRation(), int64(1000))

	volume = VolumeMap{"/data": 1000, "/data1": 1000, "/data2": 1002}
	initVolume := VolumeMap{"/data": 1000, "/data1": 1001, "/data2": 1001}
	used, unused := volume.SplitByUsed(initVolume)
	assert.Equal(t, used, VolumeMap{"/data1": 1000, "/data2": 1002})
	assert.Equal(t, unused, VolumeMap{"/data": 1000})
}

func TestVolumePlan(t *testing.T) {
	plan := VolumePlan{
		MustToVolumeBinding("AUTO:/data0:rw:100"):  VolumeMap{"/dir0": 100},
		MustToVolumeBinding("AUTO:/data1:ro:2000"): VolumeMap{"/dir1": 2000},
	}
	assert.Equal(t, plan.IntoVolumeMap(), VolumeMap{"/dir0": 100, "/dir1": 2000})

	literal := map[string]map[string]int64{
		"AUTO:/data0:rw:100":  {"/dir0": 100},
		"AUTO:/data1:ro:2000": {"/dir1": 2000},
	}
	assert.Equal(t, MustToVolumePlan(literal), plan)
	assert.Equal(t, plan.ToLiteral(), literal)

	assert.True(t, plan.Compatible(VolumePlan{
		MustToVolumeBinding("AUTO:/data0:ro:200"): VolumeMap{"/dir0": 200},
		MustToVolumeBinding("AUTO:/data1:rw:100"): VolumeMap{"/dir1": 100},
	}))
	assert.False(t, plan.Compatible(VolumePlan{
		MustToVolumeBinding("AUTO:/data0:ro:200"): VolumeMap{"/dir0": 200},
		MustToVolumeBinding("AUTO:/data1:rw:100"): VolumeMap{"/dir2": 100},
	}))

	p := MustToVolumePlan(map[string]map[string]int64{
		"AUTO:/data1:w:100": {
			"/tmp1": 100,
		},
		"AUTO:/data2:w:100": {
			"/tmp2": 100,
		},
	})
	_, _, found := p.FindAffinityPlan(MustToVolumeBinding("AUTO:/data2:w"))
	assert.True(t, found)
	_, _, found = p.FindAffinityPlan(MustToVolumeBinding("AUTO:/data1:w"))
	assert.True(t, found)
	_, _, found = p.FindAffinityPlan(MustToVolumeBinding("AUTO:/data1:rw"))
	assert.False(t, found)
	_, _, found = p.FindAffinityPlan(MustToVolumeBinding("AUTO:/data3:w"))
	assert.False(t, found)
}
