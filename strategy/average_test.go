package strategy

import (
	"sort"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestAveragePlan(t *testing.T) {
	// 正常的
	nodes := deployedNodes()
	r, err := AveragePlan(nodes, 1, 0, 0, types.ResourceAll)
	assert.NoError(t, err)
	finalCounts := []int{}
	for _, node := range nodes {
		finalCounts = append(finalCounts, node.Count+r[node.Nodename])
	}
	sort.Ints(finalCounts)
	assert.ElementsMatch(t, []int{3, 4, 6, 8}, finalCounts)

	// nodes len < limit
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 100, 0, 5, types.ResourceAll)
	assert.Error(t, err)
	// 超过 cap
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 100, 0, 0, types.ResourceAll)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough capacity")
	// 正常 limit
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 1, 1, 1, types.ResourceAll)
	assert.NoError(t, err)
}
