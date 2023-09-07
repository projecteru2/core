package strategy

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDrainedPlan(t *testing.T) {
	nodes := []Info{
		{
			Nodename: "n1",
			Capacity: 10,
			Count:    2,
		},
		{
			Nodename: "n2",
			Capacity: 9,
			Count:    3,
		},
		{
			Nodename: "n3",
			Capacity: 10,
			Count:    5,
		},
		{
			Nodename: "n4",
			Capacity: 8,
			Count:    7,
		},
	}

	r, err := DrainedPlan(context.Background(), nodes, 1, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 3, 5, 8}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(context.Background(), nodes, 2, 1, 0)
	assert.Error(t, err)

	r, err = DrainedPlan(context.Background(), nodes, 2, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 3, 5, 9}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(context.Background(), nodes, 3, 100, 0)
	assert.ElementsMatch(t, []int{2, 3, 5, 10}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(context.Background(), nodes, 10, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{2, 5, 5, 15}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(context.Background(), nodes, 25, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{10, 12, 5, 15}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(context.Background(), nodes, 29, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 12, 7, 15}, getFinalStatus(r, nodes))

	r, err = DrainedPlan(context.Background(), nodes, 37, 100, 0)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []int{12, 12, 15, 15}, getFinalStatus(r, nodes))
}
