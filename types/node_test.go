package types

import (
	"context"
	"encoding/json"
	"math"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNode(t *testing.T) {
	mockEngine := &enginemocks.API{}
	r := &enginetypes.Info{ID: "test"}
	mockEngine.On("Info", mock.Anything).Return(r, nil)

	node := &Node{}
	assert.Nil(t, node.Volume)
	assert.Nil(t, node.InitVolume)
	node.Init()
	assert.Equal(t, node.Volume, VolumeMap{})
	assert.Equal(t, node.InitVolume, VolumeMap{})
	ctx := context.Background()
	_, err := node.Info(ctx)
	assert.Error(t, err)

	node.Engine = mockEngine
	info, err := node.Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, info.ID, "test")

	node.CPUUsed = 0.0
	node.SetCPUUsed(1.0, IncrUsage)
	assert.Equal(t, node.CPUUsed, 1.0)
	node.SetCPUUsed(1.0, DecrUsage)
	assert.Equal(t, node.CPUUsed, 0.0)

	node.SetVolumeUsed(100, IncrUsage)
	assert.Equal(t, node.VolumeUsed, int64(100))
	node.SetVolumeUsed(10, DecrUsage)
	assert.Equal(t, node.VolumeUsed, int64(90))
}

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

func TestGetNUMANode(t *testing.T) {
	node := &Node{
		NUMA: NUMA{"1": "node1", "2": "node2", "3": "node1", "4": "node2"},
	}
	cpu := CPUMap{"1": 100, "2": 100}
	nodeID := node.GetNUMANode(cpu)
	assert.Equal(t, nodeID, "")
	cpu = CPUMap{"1": 100, "3": 100}
	nodeID = node.GetNUMANode(cpu)
	assert.Equal(t, nodeID, "node1")
	cpu = nil
	nodeID = node.GetNUMANode(cpu)
	assert.Equal(t, nodeID, "")
}

func TestSetNUMANodeMemory(t *testing.T) {
	node := &Node{
		NUMAMemory: NUMAMemory{"n1": 100},
	}
	// incr
	node.IncrNUMANodeMemory("n1", 1)
	assert.Len(t, node.NUMAMemory, 1)
	assert.Equal(t, node.NUMAMemory["n1"], int64(101))
	// decr
	node.DecrNUMANodeMemory("n1", 1)
	assert.Len(t, node.NUMAMemory, 1)
	assert.Equal(t, node.NUMAMemory["n1"], int64(100))
}

func TestStorage(t *testing.T) {
	node := &Node{
		InitStorageCap: 0,
	}
	assert.Equal(t, node.StorageUsage(), 1.0)
	assert.Equal(t, node.StorageUsed(), int64(0))
	assert.Equal(t, node.AvailableStorage(), int64(math.MaxInt64))
	node.InitStorageCap = 2
	node.StorageCap = 1
	assert.Equal(t, node.StorageUsage(), 0.5)
	assert.Equal(t, node.StorageUsed(), int64(1))
	assert.Equal(t, node.AvailableStorage(), int64(1))
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

func TestNodeInfoGetResource(t *testing.T) {
	n := NodeInfo{
		Usages: map[ResourceType]float64{
			ResourceCPU:     0.1,
			ResourceMemory:  0.2,
			ResourceStorage: 0.3,
			ResourceVolume:  0.4,
		},
	}
	assert.InDelta(t, n.GetResourceUsage(ResourceCPU), 0.1, 0.000001)
	assert.InDelta(t, n.GetResourceUsage(ResourceCPU|ResourceMemory), 0.3, 0.000001)
	assert.InDelta(t, n.GetResourceUsage(ResourceVolume|ResourceMemory), 0.6, 0.000001)
	assert.InDelta(t, n.GetResourceUsage(ResourceAll), 1.0, 0.000001)

	n = NodeInfo{
		Rates: map[ResourceType]float64{
			ResourceCPU:     0.1,
			ResourceMemory:  0.2,
			ResourceStorage: 0.3,
			ResourceVolume:  0.4,
		},
	}
	assert.InDelta(t, n.GetResourceRate(ResourceCPU), 0.1, 0.000001)
	assert.InDelta(t, n.GetResourceRate(ResourceCPU|ResourceMemory), 0.3, 0.000001)
	assert.InDelta(t, n.GetResourceRate(ResourceVolume|ResourceMemory), 0.6, 0.000001)
	assert.InDelta(t, n.GetResourceRate(ResourceAll), 1.0, 0.000001)
}
