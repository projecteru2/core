package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDrainedPlan(t *testing.T) {
	nodes := genNodesByCapCount([]int{10, 9, 10, 8}, []int{2, 3, 5, 7})

	r, err := DrainedPlan(t.Context(), nodes, 1, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 3, 5, 8}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(t.Context(), nodes, 2, 1, 0)
	assert.Error(t, err)

	r, err = DrainedPlan(t.Context(), nodes, 2, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 3, 5, 9}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(t.Context(), nodes, 3, 100, 0)
	assert.ElementsMatch(t, []int{2, 3, 5, 10}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(t.Context(), nodes, 10, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 5, 5, 15}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(t.Context(), nodes, 25, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{10, 12, 5, 15}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(t.Context(), nodes, 29, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 12, 7, 15}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(t.Context(), nodes, 37, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 12, 15, 15}, getFinalStatus(r, nodes))
}

func TestDrainedPlanOrdersByCapacityThenUsage(t *testing.T) {
	nodes := []Info{
		{Nodename: "small-idle", Capacity: 5, Usage: 0.1},
		{Nodename: "big-busy", Capacity: 10, Usage: 0.9},
	}

	r, err := DrainedPlan(t.Context(), nodes, 5, 15, 0)
	assert.NoError(t, err)
	assert.Equal(t, map[string]int{"small-idle": 5}, r)
}
