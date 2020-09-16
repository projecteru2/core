package strategy

import (
	"math/rand"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/stretchr/testify/assert"
)

func TestCommunismPlan(t *testing.T) {
	nodes := deployedNodes()
	r, err := CommunismPlan(nodes, 1, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 1)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 2, 1, 0, types.ResourceAll)
	assert.Error(t, err)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 2, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 2)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 3, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 2)
	assert.Equal(t, r[1].Deploy, 1)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 4, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 3)
	assert.Equal(t, r[1].Deploy, 1)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 29, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 10)
	assert.Equal(t, r[1].Deploy, 9)
	assert.Equal(t, r[2].Deploy, 6)
	assert.Equal(t, r[3].Deploy, 4)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 37, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 40, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
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

func Benchmark_CommunismPlan(b *testing.B) {
	b.StopTimer()
	var count = 10000
	var maxDeployed = 1024
	var volTotal = maxDeployed * count
	var need = volTotal - 1
	// Simulate `count` nodes with difference deploy status, each one can deploy `maxDeployed` containers
	// and then we deploy `need` containers
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		hugePod := utils.GenerateNodes(count, 1, 1, 0, 10)
		hugePod = randomDeployStatus(hugePod, maxDeployed)
		b.StartTimer()
		_, err := CommunismPlan(hugePod, need, 100, 0, types.ResourceAll)
		b.StopTimer()
		assert.NoError(b, err)
	}
}
