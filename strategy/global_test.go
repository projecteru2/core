package strategy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGlobalPlan1(t *testing.T) {
	n1 := Info{
		Nodename: "n1",
		Usage:    0.8,
		Rate:     0.05,
		Capacity: 1,
	}
	n2 := Info{
		Nodename: "n2",
		Usage:    0.5,
		Rate:     0.11,
		Capacity: 1,
	}
	n3 := Info{
		Nodename: "n3",
		Usage:    2.2,
		Rate:     0.05,
		Capacity: 1,
	}
	arg := []Info{n3, n2, n1}
	r, err := GlobalPlan(arg, 3, 100, 0)
	assert.NoError(t, err)
	assert.Equal(t, r[arg[0].Nodename], 1)
}

func TestGlobalPlan2(t *testing.T) {
	n1 := Info{
		Nodename: "n1",
		Usage:    1.6,
		Rate:     0.05,
		Capacity: 100,
	}
	n2 := Info{
		Nodename: "n2",
		Usage:    0.5,
		Rate:     0.11,
		Capacity: 100,
	}
	arg := []Info{n2, n1}
	r, err := GlobalPlan(arg, 2, 100, 0)
	assert.NoError(t, err)
	assert.Equal(t, r[arg[0].Nodename], 2)
}

func TestGlobalPlan3(t *testing.T) {
	n1 := Info{
		Nodename: "n1",
		Usage:    0.5259232954545454,
		Rate:     0.0000712,
		Capacity: 100,
	}

	r, err := GlobalPlan([]Info{n1}, 1, 100, 0)
	assert.NoError(t, err)
	assert.Equal(t, r["n1"], 1)
}

func TestGlobal3(t *testing.T) {
	_, err := GlobalPlan([]Info{}, 10, 1, 0)
	assert.Error(t, err)
	nodeInfo := Info{
		Nodename: "n1",
		Usage:    1,
		Rate:     0.3,
		Capacity: 100,
		Count:    21,
	}
	r, err := GlobalPlan([]Info{nodeInfo}, 10, 100, 0)
	assert.NoError(t, err)
	assert.Equal(t, r["n1"], 10)
}
