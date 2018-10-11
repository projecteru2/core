package complexscheduler

import (
	"sort"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestFillPlan(t *testing.T) {
	// 正常的全量补充
	n := 10
	nodes := deployedNodes()
	resultCap := []int{}
	resultDeploy := []int{}
	for i := range nodes {
		resultCap = append(resultCap, nodes[i].Capacity-n+nodes[i].Count)
		resultDeploy = append(resultDeploy, n-nodes[i].Count)
	}
	r, err := FillPlan(nodes, n, 0)
	assert.NoError(t, err)
	sort.Slice(r, func(i, j int) bool { return r[i].Count < r[j].Count })
	for i := range r {
		assert.Equal(t, r[i].Capacity, resultCap[i])
		assert.Equal(t, r[i].Deploy, resultDeploy[i])
	}

	// 局部补充
	n = 5
	nodes = deployedNodes()
	resultCap = []int{}
	resultDeploy = []int{}
	for i := range nodes {
		if nodes[i].Count >= n {
			continue
		}
		resultCap = append(resultCap, nodes[i].Capacity-n+nodes[i].Count)
		resultDeploy = append(resultDeploy, n-nodes[i].Count)
	}
	r, err = FillPlan(nodes, n, 0)
	assert.NoError(t, err)
	sort.Slice(r, func(i, j int) bool { return r[i].Count < r[j].Count })
	for i := range r {
		assert.Equal(t, r[i].Capacity, resultCap[i])
		assert.Equal(t, r[i].Deploy, resultDeploy[i])
	}

	// 局部补充不能
	n = 15
	nodes = deployedNodes()
	_, err = FillPlan(nodes, n, 0)
	assert.EqualError(t, types.IsDetailedErr(err), types.ErrInsufficientRes.Error())

	// 全局补充不能
	n = 1
	nodes = deployedNodes()
	_, err = FillPlan(nodes, n, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "each node has enough containers")
}
