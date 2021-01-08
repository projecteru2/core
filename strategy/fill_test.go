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
	r, err := FillPlan(nodes, n, 0, 0, types.ResourceAll)
	assert.NoError(t, err)
	finalCounts := []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	sort.Ints(finalCounts)
	assert.ElementsMatch(t, []int{10, 10, 10, 10}, finalCounts)

	// 局部补充
	n = 5
	nodes = deployedNodes()
	r, err = FillPlan(nodes, n, 0, 0, types.ResourceAll)
	assert.NoError(t, err)
	finalCounts = []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	sort.Ints(finalCounts)
	assert.ElementsMatch(t, []int{5, 5, 5, 7}, finalCounts)

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
	assert.Contains(t, err.Error(), "each node has enough workloads")

	// LimitNode
	n = 10
	nodes = deployedNodes()
	_, err = FillPlan(nodes, n, 0, 2, types.ResourceAll)
	assert.NoError(t, err)

	// 局部补充
	n = 1
	nodes = []Info{
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
