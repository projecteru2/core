package complexscheduler

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestAveragePlan(t *testing.T) {
	// 正常的
	nodes := deployedNodes()
	originCap := []int{}
	for i := range nodes {
		originCap = append(originCap, nodes[i].Capacity)
	}
	r, err := AveragePlan(nodes, 1, 0, types.ResourceAll)
	assert.NoError(t, err)
	for i := range r {
		assert.Equal(t, r[i].Deploy, 1)
		assert.Equal(t, r[i].Capacity, originCap[i]-1)
	}
	// nodes len < limit
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 100, 5, types.ResourceAll)
	assert.Error(t, err)
	// 超过 cap
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 100, 0, types.ResourceAll)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough capacity")
	// 正常 limit
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 1, 1, types.ResourceAll)
	assert.NoError(t, err)
}
