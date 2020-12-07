package strategy

import (
	"math/rand"
	"testing"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/stretchr/testify/assert"
)

func TestCommunismPlan(t *testing.T) {
	nodes := deployedNodes()
	r, err := CommunismPlan(nodes, 1, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[nodes[0].Nodename], 1)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 2, 1, 0, types.ResourceAll)
	assert.Error(t, err)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 2, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[nodes[0].Nodename], 2)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 3, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[nodes[0].Nodename], 2)
	assert.Equal(t, r[nodes[1].Nodename], 1)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 4, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[nodes[0].Nodename], 3)
	assert.Equal(t, r[nodes[1].Nodename], 1)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 29, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[nodes[0].Nodename], 10)
	assert.Equal(t, r[nodes[1].Nodename], 9)
	assert.Equal(t, r[nodes[2].Nodename], 6)
	assert.Equal(t, r[nodes[3].Nodename], 4)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 37, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 40, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
}

func randomDeployStatus(scheduleInfos []resourcetypes.ScheduleInfo, maxDeployed int) (sis []Info) {
	s := rand.NewSource(int64(1024))
	r := rand.New(s)
	for range scheduleInfos {
		sis = append(sis, Info{
			Capacity: maxDeployed,
			Count:    r.Intn(maxDeployed),
		})
	}
	return
}

func Benchmark_CommunismPlan(b *testing.B) {
	b.StopTimer()
	var count = 10000
	var maxDeployed = 1024
	var volTotal = maxDeployed * count
	var need = volTotal - 1
	// Simulate `count` nodes with difference deploy status, each one can deploy `maxDeployed` workloads
	// and then we deploy `need` workloads
	for i := 0; i < b.N; i++ {
		// 24 core, 128G memory, 10 pieces per core
		t := utils.GenerateScheduleInfos(count, 1, 1, 0, 10)
		hugePod := randomDeployStatus(t, maxDeployed)
		b.StartTimer()
		_, err := CommunismPlan(hugePod, need, 100, 0, types.ResourceAll)
		b.StopTimer()
		assert.NoError(b, err)
	}
}
