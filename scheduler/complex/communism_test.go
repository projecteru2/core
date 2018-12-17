package complexscheduler

import (
	"math/rand"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func deployedNodes() []types.NodeInfo {
	return []types.NodeInfo{
		{
			Name:     "n1",
			Capacity: 10,
			Count:    2,
		},
		{
			Name:     "n2",
			Capacity: 10,
			Count:    3,
		},
		{
			Name:     "n3",
			Capacity: 10,
			Count:    5,
		},
		{
			Name:     "n4",
			Capacity: 10,
			Count:    7,
		},
	}
}

func TestCommunismDivisionPlan(t *testing.T) {
	nodes := deployedNodes()
	r, err := CommunismDivisionPlan(nodes, 1)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 1)
	nodes = deployedNodes()
	r, err = CommunismDivisionPlan(nodes, 2)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 2)
	nodes = deployedNodes()
	r, err = CommunismDivisionPlan(nodes, 3)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 3)
	nodes = deployedNodes()
	r, err = CommunismDivisionPlan(nodes, 4)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 3)
	assert.Equal(t, r[1].Deploy, 1)
}

func randomDeployStatus(nodesInfo []types.NodeInfo, maxDeployed int) []types.NodeInfo {
	s := rand.NewSource(int64(1024))
	r := rand.New(s)
	for i := range nodesInfo {
		nodesInfo[i].Capacity = maxDeployed
		nodesInfo[i].Count = r.Intn(maxDeployed)

	}
	return nodesInfo
}

func Benchmark_CommunismDivisionPlan(b *testing.B) {
	b.StopTimer()
	var count = 10000
	var maxDeployed = 1024
	var volTotal = maxDeployed * count
	var need = volTotal - 1
	// Simulate `count` nodes with difference deploy status, each one can deploy `maxDeployed` containers
	// and then we deploy `need` containers
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		hugePod := generateNodes(count, 1, 1, 10)
		hugePod = randomDeployStatus(hugePod, maxDeployed)
		b.StartTimer()
		_, err := CommunismDivisionPlan(hugePod, need)
		b.StopTimer()
		assert.NoError(b, err)
	}
}
