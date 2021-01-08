package strategy

import (
	"math/rand"
	"sort"
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
	counts := []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{3, 3, 5, 7}, counts)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 2, 1, 0, types.ResourceAll)
	assert.Error(t, err)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 2, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	counts = []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{3, 4, 5, 7}, counts)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 3, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	counts = []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{4, 4, 5, 7}, counts)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 4, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	counts = []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{4, 5, 5, 7}, counts)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 29, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	counts = []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{11, 11, 12, 12}, counts)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 37, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	counts = []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{12, 13, 14, 15}, counts)
	nodes = deployedNodes()
	r, err = CommunismPlan(nodes, 40, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	counts = []int{r[nodes[0].Nodename] + nodes[0].Count, r[nodes[1].Nodename] + nodes[1].Count, r[nodes[2].Nodename] + nodes[2].Count, r[nodes[3].Nodename] + nodes[3].Count}
	sort.Ints(counts)
	assert.ElementsMatch(t, []int{12, 13, 15, 17}, counts)
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

func TestCommunismPlanCapacityPriority(t *testing.T) {
	nodes := []Info{
		{
			Nodename: "n1",
			Capacity: 1,
			Count:    0,
		},
		{
			Nodename: "n2",
			Capacity: 2,
			Count:    0,
		},
		{
			Nodename: "n3",
			Capacity: 1,
			Count:    0,
		},
		{
			Nodename: "n4",
			Capacity: 5,
			Count:    0,
		},
		{
			Nodename: "n5",
			Capacity: 10,
			Count:    0,
		},
	}
	deploy, err := CommunismPlan(nodes, 3, 15, 0, types.ResourceAll)
	assert.Nil(t, err)
	total := 0
	for _, d := range deploy {
		total += d
	}
	assert.EqualValues(t, 3, total)
}
