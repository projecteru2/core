package strategy

import (
	"errors"
	"slices"
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestFillPlan(t *testing.T) {
	n := 10
	nodes := deployedNodes()
	r, err := FillPlan(t.Context(), nodes, n, 0, 0)
	assert.NoError(t, err)
	finalCounts := []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	slices.Sort(finalCounts)
	assert.ElementsMatch(t, []int{10, 10, 10, 10}, finalCounts)

	n = 5
	nodes = deployedNodes()
	r, err = FillPlan(t.Context(), nodes, n, 0, 0)
	assert.NoError(t, err)
	finalCounts = []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	slices.Sort(finalCounts)
	assert.ElementsMatch(t, []int{5, 5, 5, 7}, finalCounts)

	n = 15
	nodes = deployedNodes()
	_, err = FillPlan(t.Context(), nodes, n, 0, 0)
	assert.True(t, errors.Is(err, types.ErrInsufficientResource))

	n = 1
	nodes = deployedNodes()
	_, err = FillPlan(t.Context(), nodes, n, 0, 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "each node has enough workloads")

	n = 10
	nodes = deployedNodes()
	_, err = FillPlan(t.Context(), nodes, n, 0, 2)
	assert.NoError(t, err)

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

	_, err = FillPlan(t.Context(), nodes, n, 0, 3)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot alloc a fill node plan")

	nodes = genNodesByCapCount([]int{1, 2, 3, 4, 5}, []int{3, 3, 3, 3, 3})
	r, err = FillPlan(t.Context(), nodes, 4, 0, 3)
	assert.Nil(t, err)
	assert.ElementsMatch(t, []int{3, 3, 4, 4, 4}, getFinalStatus(r, nodes))
	assert.EqualValues(t, 1, r["4"])
	assert.EqualValues(t, 1, r["3"])
	assert.EqualValues(t, 1, r["2"])

	_, err = FillPlan(t.Context(), nodes, 5, 1000, 0)
	assert.Contains(t, err.Error(), "not enough nodes that can fill up to 5 instances, require 1 nodes")
}
