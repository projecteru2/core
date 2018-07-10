package complexscheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAveragePlan(t *testing.T) {
	// 正常的
	nodes := deployedNodes()
	originCap := []int{}
	for i := range nodes {
		originCap = append(originCap, nodes[i].Capacity)
	}
	r, err := AveragePlan(nodes, 1)
	assert.NoError(t, err)
	for i := range r {
		assert.Equal(t, r[i].Deploy, 1)
		assert.Equal(t, r[i].Capacity, originCap[i]-1)
	}

	// 超过 cap
	nodes = deployedNodes()
	_, err = AveragePlan(nodes, 100)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enough capacity")

}
