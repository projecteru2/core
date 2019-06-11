package types

import (
	"context"
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
}

func TestCPUMap(t *testing.T) {
	cpuMap := CPUMap{"0": 50, "1": 70}
	total := cpuMap.Total()
	assert.Equal(t, total, 120)

	cpuMap.Add(CPUMap{"0": 20})
	assert.Equal(t, cpuMap["0"], 70)

	cpuMap.Add(CPUMap{"3": 100})
	assert.Equal(t, cpuMap["3"], 100)

	cpuMap.Sub(CPUMap{"1": 20})
	assert.Equal(t, cpuMap["1"], 50)

	m := cpuMap.Map()
	assert.Equal(t, m["1"], 50)
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
