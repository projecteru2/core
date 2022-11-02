package strategy

import (
	"context"
	"sort"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestAveragePlan(t *testing.T) {
	// 正常的
	nodes := deployedNodes()
	r, err := AveragePlan(context.Background(), nodes, 1, 0, 0)
	assert.NoError(t, err)
	finalCounts := []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	sort.Ints(finalCounts)
	assert.ElementsMatch(t, []int{3, 4, 6, 8}, finalCounts)

	// nodes len < limit
	nodes = deployedNodes()
	_, err = AveragePlan(context.Background(), nodes, 100, 0, 5)
	assert.Error(t, err)
	// 超过 cap
	nodes = deployedNodes()
	_, err = AveragePlan(context.Background(), nodes, 100, 0, 0)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, types.ErrInsufficientCapacity))
	// 正常 limit
	nodes = deployedNodes()
	_, err = AveragePlan(context.Background(), nodes, 1, 1, 1)
	assert.NoError(t, err)

	nodes = genNodesByCapCount([]int{1, 2, 3, 4, 5}, []int{3, 3, 3, 3, 3})
	_, err = AveragePlan(context.Background(), nodes, 4, 100, 4)
	assert.Contains(t, err.Error(), "not enough nodes with capacity of 4, require 4 nodes")

	nodes = genNodesByCapCount([]int{1, 2, 3, 4, 5}, []int{3, 3, 3, 3, 3})
	_, err = AveragePlan(context.Background(), nodes, 2, 100, 0)
	assert.Contains(t, err.Error(), "not enough nodes with capacity of 2, require 5 nodes")
}
