package types

import (
	"context"
	"math"
	"reflect"
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

func TestGetNUMANode(t *testing.T) {
	node := &Node{
		NodeMeta: NodeMeta{NUMA: NUMA{"1": "node1", "2": "node2", "3": "node1", "4": "node2"}},
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
		NodeMeta: NodeMeta{NUMAMemory: NUMAMemory{"n1": 100}},
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
		NodeMeta: NodeMeta{InitStorageCap: 0},
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

func TestNodeUsage(t *testing.T) {
	node := Node{
		NodeMeta: NodeMeta{
			CPU:            CPUMap{"0": 100, "1": 50},
			InitCPU:        CPUMap{"0": 200, "1": 200},
			Volume:         VolumeMap{"/data1": 1000, "/data2": 2000},
			InitVolume:     VolumeMap{"/data1": 1000, "/data2": 2500},
			MemCap:         1,
			InitMemCap:     100,
			StorageCap:     0,
			InitStorageCap: 2,
		},
		CPUUsed:    1.5,
		VolumeUsed: 500,
	}
	usages := node.ResourceUsages()
	assert.EqualValues(t, 1, usages[ResourceStorage])
	assert.EqualValues(t, 0.99, usages[ResourceMemory])
	assert.EqualValues(t, 0.75, usages[ResourceCPU])
	assert.EqualValues(t, 500./3500., usages[ResourceVolume])
}

func TestAddNodeOptions(t *testing.T) {
	o := AddNodeOptions{
		Volume: VolumeMap{"/data1": 1, "/data2": 2},
	}
	o.Normalize()
	assert.EqualValues(t, 3, o.Storage)
}

func TestNodeWithResource(t *testing.T) {
	n := Node{
		NodeMeta: NodeMeta{
			CPU:    CPUMap{"0": 0},
			Volume: VolumeMap{"sda1": 0},
		},
	}
	resource := &ResourceMeta{
		CPUQuotaLimit:     0.4,
		CPUQuotaRequest:   0.3,
		CPU:               CPUMap{"0": 30},
		MemoryLimit:       100,
		MemoryRequest:     99,
		StorageLimit:      88,
		StorageRequest:    87,
		VolumePlanLimit:   MustToVolumePlan(map[string]map[string]int64{"AUTO:/data0:rw:100": {"/sda0": 100}}),
		VolumePlanRequest: MustToVolumePlan(map[string]map[string]int64{"AUTO:/data1:rw:101": {"sda1": 101}}),
		NUMANode:          "0",
	}
	n.RecycleResources(resource)
	assert.EqualValues(t, -0.3, n.CPUUsed)
	assert.True(t, reflect.DeepEqual(n.CPU, CPUMap{"0": 30}))
	assert.EqualValues(t, 99, n.MemCap)
	assert.EqualValues(t, 87, n.StorageCap)
	assert.EqualValues(t, -101, n.VolumeUsed)

	n.PreserveResources(resource)
	assert.EqualValues(t, 0, n.CPUUsed)
	assert.True(t, reflect.DeepEqual(n.CPU, CPUMap{"0": 0}))
	assert.EqualValues(t, 0, n.MemCap)
	assert.EqualValues(t, 0, n.StorageCap)
	assert.EqualValues(t, 0, n.VolumeUsed)
}
