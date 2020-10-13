package strategy

import (
	"errors"
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
	r, err := FillPlan(nodes, n, 0, 0, types.ResourceAll)
	assert.NoError(t, err)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Count < nodes[j].Count })
	for i := range nodes {
		assert.Equal(t, nodes[i].Capacity, resultCap[i])
		assert.Equal(t, r[nodes[i].Nodename].Deploy, resultDeploy[i])
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
	r, err = FillPlan(nodes, n, 0, 0, types.ResourceAll)
	assert.NoError(t, err)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].Count < nodes[j].Count })
	i := 0
	for _, v := range nodes {
		if v.Capacity >= n {
			continue
		}
		assert.Equal(t, nodes[i].Capacity, resultCap[i])
		assert.Equal(t, r[nodes[i].Nodename].Deploy, resultDeploy[i])
		i++
	}

	// 局部补充不能
	n = 15
	nodes = deployedNodes()
	_, err = FillPlan(nodes, n, 0, 0, types.ResourceAll)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	// 全局补充不能
	n = 1
	nodes = deployedNodes()
	_, err = FillPlan(nodes, n, 0, 0, types.ResourceAll)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "each node has enough containers")

	// LimitNode
	n = 10
	nodes = deployedNodes()
	_, err = FillPlan(nodes, n, 0, 2, types.ResourceAll)
	assert.NoError(t, err)

	// 局部补充
	n = 1
	nodes = []types.StrategyInfo{
		{
			Nodename: "65",
			Capacity: 0,
			Count:    0,
		},
		{
			Nodename: "67",
			Capacity: 10,
			Count:    0,
		},
	}

	_, err = FillPlan(nodes, n, 0, 3, types.ResourceAll)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot alloc a fill node plan")
}
