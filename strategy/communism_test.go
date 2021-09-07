package strategy

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"testing"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/utils"

	"github.com/stretchr/testify/assert"
)

func TestCommunismPlan(t *testing.T) {
	nodes := deployedNodes()
	r, err := CommunismPlan(context.TODO(), nodes, 1, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{3, 3, 5, 7}, getFinalStatus(r, nodes))

	r, err = CommunismPlan(context.TODO(), nodes, 2, 1, 0)
	assert.Error(t, err)

	r, err = CommunismPlan(context.TODO(), nodes, 2, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{3, 4, 5, 7}, getFinalStatus(r, nodes))

	r, err = CommunismPlan(context.TODO(), nodes, 3, 100, 0)
	assert.ElementsMatch(t, []int{4, 4, 5, 7}, getFinalStatus(r, nodes))

	r, err = CommunismPlan(context.TODO(), nodes, 4, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{4, 5, 5, 7}, getFinalStatus(r, nodes))

	r, err = CommunismPlan(context.TODO(), nodes, 29, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{11, 11, 12, 12}, getFinalStatus(r, nodes))

	r, err = CommunismPlan(context.TODO(), nodes, 37, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 13, 14, 15}, getFinalStatus(r, nodes))

	r, err = CommunismPlan(context.TODO(), nodes, 40, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 13, 15, 17}, getFinalStatus(r, nodes))
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
		_, err := CommunismPlan(context.TODO(), hugePod, need, 100, 0)
		b.StopTimer()
		assert.NoError(b, err)
	}
}
func genNodesByCapCount(caps, counts []int) (infos []Info) {
	for i := range caps {
		infos = append(infos, Info{
			Nodename: fmt.Sprintf("%d", i),
			Capacity: caps[i],
			Count:    counts[i],
		})
	}
	return
}

func getFinalStatus(deploy map[string]int, infos []Info) (counts []int) {
	for _, info := range infos {
		counts = append(counts, info.Count+deploy[info.Nodename])
	}
	sort.Ints(counts)
	return
}

func TestCommunismPlanCapacityPriority(t *testing.T) {

	nodes := genNodesByCapCount([]int{1, 2, 1, 5, 10}, []int{0, 0, 0, 0, 0})
	deploy, err := CommunismPlan(context.TODO(), nodes, 3, 15, 0)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []int{0, 0, 1, 1, 1}, getFinalStatus(deploy, nodes))
	assert.EqualValues(t, 1, deploy["1"])
	assert.EqualValues(t, 1, deploy["3"])
	assert.EqualValues(t, 1, deploy["4"])

	nodes = genNodesByCapCount([]int{10, 4, 4}, []int{1, 1, 10})
	deploy, err = CommunismPlan(context.TODO(), nodes, 5, 100, 0)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []int{3, 4, 10}, getFinalStatus(deploy, nodes))
	assert.EqualValues(t, 3, deploy["0"])
	assert.EqualValues(t, 2, deploy["1"])

	nodes = genNodesByCapCount([]int{4, 5, 4, 10}, []int{2, 2, 4, 0})
	deploy, err = CommunismPlan(context.TODO(), nodes, 3, 100, 0)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []int{2, 2, 3, 4}, getFinalStatus(deploy, nodes))
	assert.EqualValues(t, 3, deploy["3"])

	nodes = genNodesByCapCount([]int{3, 4, 5, 10}, []int{0, 0, 0, 0})
	deploy, err = CommunismPlan(context.TODO(), nodes, 3, 100, 0)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []int{0, 1, 1, 1}, getFinalStatus(deploy, nodes))
	assert.EqualValues(t, 1, deploy["3"])
	assert.EqualValues(t, 1, deploy["2"])
	assert.EqualValues(t, 1, deploy["1"])

	// test limit
	nodes = genNodesByCapCount([]int{3, 4, 5, 10}, []int{3, 5, 7, 10})
	deploy, err = CommunismPlan(context.TODO(), nodes, 3, 10, 5)
	assert.EqualError(t, err, "reached nodelimit, a node can host at most 5 instances: not enough resource")
	deploy, err = CommunismPlan(context.TODO(), nodes, 3, 10, 6)
	assert.Nil(t, err)
}
