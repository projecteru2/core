package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestGlobalPlan1(t *testing.T) {
	n1 := Info{
		Nodename: "n1",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    0.1,
			types.ResourceMemory: 0.7,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.03,
			types.ResourceMemory: 0.02,
		},
		Capacity: 1,
	}
	n2 := Info{
		Nodename: "n2",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    0.2,
			types.ResourceMemory: 0.3,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.04,
			types.ResourceMemory: 0.07,
		},
		Capacity: 1,
	}
	n3 := Info{
		Nodename: "n3",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    1.3,
			types.ResourceMemory: 0.9,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.01,
			types.ResourceMemory: 0.04,
		},
		Capacity: 1,
	}
	arg := []Info{n3, n2, n1}
	r, err := GlobalPlan(arg, 3, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[arg[0].Nodename], 1)
}

func TestGlobalPlan2(t *testing.T) {
	n1 := Info{
		Nodename: "n1",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    0.9,
			types.ResourceMemory: 0.7,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.03,
			types.ResourceMemory: 0.02,
		},
		Capacity: 100,
	}
	n2 := Info{
		Nodename: "n2",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    0.2,
			types.ResourceMemory: 0.3,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.04,
			types.ResourceMemory: 0.07,
		},
		Capacity: 100,
	}
	arg := []Info{n2, n1}
	r, err := GlobalPlan(arg, 2, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r[arg[0].Nodename], 2)
}

func TestGlobalPlan3(t *testing.T) {
	n1 := Info{
		Nodename: "n1",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    0.9,
			types.ResourceMemory: 0.5259232954545454,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.03,
			types.ResourceMemory: 0.0000712,
		},
		Capacity: 100,
	}

	r, err := GlobalPlan([]Info{n1}, 1, 100, 0, types.ResourceMemory)
	assert.NoError(t, err)
	assert.Equal(t, r["n1"], 1)
}

func TestGlobal3(t *testing.T) {
	_, err := GlobalPlan([]Info{}, 10, 1, 0, types.ResourceAll)
	assert.Error(t, err)
	nodeInfo := Info{
		Nodename: "n1",
		Usages: map[types.ResourceType]float64{
			types.ResourceCPU:    0.7,
			types.ResourceMemory: 0.3,
		},
		Rates: map[types.ResourceType]float64{
			types.ResourceCPU:    0.1,
			types.ResourceMemory: 0.2,
		},
		Capacity: 100,
		Count:    21,
	}
	r, err := GlobalPlan([]Info{nodeInfo}, 10, 100, 0, types.ResourceAll)
	assert.NoError(t, err)
	assert.Equal(t, r["n1"], 10)
}
