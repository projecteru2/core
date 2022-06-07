package strategy

import (
	"context"
	"errors"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestFillPlan(t *testing.T) {
	// 正常的全量补充
	n := 10
	nodes := deployedNodes()
	r, err := FillPlan(context.TODO(), nodes, n, 0, 0)
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
	r, err = FillPlan(context.TODO(), nodes, n, 0, 0)
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
	_, err = FillPlan(context.TODO(), nodes, n, 0, 0)
	assert.True(t, errors.Is(err, types.ErrInsufficientRes))

	// 全局补充不能
	n = 1
	nodes = deployedNodes()
	_, err = FillPlan(context.TODO(), nodes, n, 0, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "each node has enough workloads")

	// LimitNode
	n = 10
	nodes = deployedNodes()
	_, err = FillPlan(context.TODO(), nodes, n, 0, 2)
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

	_, err = FillPlan(context.TODO(), nodes, n, 0, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot alloc a fill node plan")

	nodes = genNodesByCapCount([]int{1, 2, 3, 4, 5}, []int{3, 3, 3, 3, 3})
	r, err = FillPlan(context.TODO(), nodes, 4, 0, 3)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []int{3, 3, 4, 4, 4}, getFinalStatus(r, nodes))
	assert.EqualValues(t, 1, r["4"])
	assert.EqualValues(t, 1, r["3"])
	assert.EqualValues(t, 1, r["2"])

	_, err = FillPlan(context.TODO(), nodes, 5, 1000, 0)
	assert.EqualError(t, err, "not enough resource: not enough nodes that can fill up to 5 instances, require 1 nodes")
}
