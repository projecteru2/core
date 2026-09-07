package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDrainedPlan(t *testing.T) {
	nodes := genNodesByCapCount([]int{10, 9, 10, 8}, []int{2, 3, 5, 7})

	tests := []struct {
		name    string
		need    int
		total   int
		wantErr bool
		want    []int
	}{
		{name: "need=1", need: 1, total: 100, want: []int{2, 3, 5, 8}},
		{name: "need=2,total=1", need: 2, total: 1, wantErr: true},
		{name: "need=2", need: 2, total: 100, want: []int{2, 3, 5, 9}},
		{name: "need=3", need: 3, total: 100, want: []int{2, 3, 5, 10}},
		{name: "need=10", need: 10, total: 100, want: []int{2, 5, 5, 15}},
		{name: "need=25", need: 25, total: 100, want: []int{10, 12, 5, 15}},
		{name: "need=29", need: 29, total: 100, want: []int{12, 12, 7, 15}},
		{name: "need=37", need: 37, total: 100, want: []int{12, 12, 15, 15}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := DrainedPlan(t.Context(), nodes, tt.need, tt.total, 0)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.ElementsMatch(t, tt.want, getFinalStatus(r, nodes))
		})
	}
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
