package strategy

import (
	"slices"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestAveragePlan(t *testing.T) {
	nodes := deployedNodes()
	r, err := AveragePlan(t.Context(), nodes, 1, 0, 0)
	assert.NoError(t, err)
	finalCounts := []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	slices.Sort(finalCounts)
	assert.ElementsMatch(t, []int{3, 4, 6, 8}, finalCounts)

	nodes = deployedNodes()
	_, err = AveragePlan(t.Context(), nodes, 100, 0, 5)
	assert.Error(t, err)
	nodes = deployedNodes()
	_, err = AveragePlan(t.Context(), nodes, 100, 0, 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, types.ErrInsufficientCapacity))
	nodes = deployedNodes()
	_, err = AveragePlan(t.Context(), nodes, 1, 1, 1)
	assert.NoError(t, err)

	nodes = genNodesByCapCount([]int{1, 2, 3, 4, 5}, []int{3, 3, 3, 3, 3})
	_, err = AveragePlan(t.Context(), nodes, 4, 100, 4)
	assert.Contains(t, err.Error(), "not enough nodes with capacity of 4, require 4 nodes")

	nodes = genNodesByCapCount([]int{1, 2, 3, 4, 5}, []int{3, 3, 3, 3, 3})
	_, err = AveragePlan(t.Context(), nodes, 2, 100, 0)
	assert.Contains(t, err.Error(), "not enough nodes with capacity of 2, require 5 nodes")
}
