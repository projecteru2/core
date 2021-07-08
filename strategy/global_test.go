package strategy

import (
	"context"
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
	r, err := GlobalPlan(context.TODO(), arg, 3, 100, 0)
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
	r, err := GlobalPlan(context.TODO(), arg, 2, 100, 0)
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

	r, err := GlobalPlan(context.TODO(), []Info{n1}, 1, 100, 0)
	assert.NoError(t, err)
	assert.Equal(t, r["n1"], 1)
}

func TestGlobal3(t *testing.T) {
	_, err := GlobalPlan(context.TODO(), []Info{}, 10, 1, 0)
	assert.Error(t, err)
	nodeInfo := Info{
		Nodename: "n1",
		Usage:    1,
		Rate:     0.3,
		Capacity: 100,
		Count:    21,
	}
	r, err := GlobalPlan(context.TODO(), []Info{nodeInfo}, 10, 100, 0)
	assert.NoError(t, err)
	assert.Equal(t, r["n1"], 10)
}

func TestGlobalIssue455(t *testing.T) {
	infos := []Info{
		{
			Nodename: "spp-qa-vm-node-1",
			Usage:    0.07999999999999996,
			Rate:     3.725290298461914e-08,
			Capacity: 10726691,
			Count:    7,
		},
		{
			Nodename: "spp-qa-vm-node-2",
			Usage:    0.24,
			Rate:     3.725290298461914e-08,
			Capacity: 4290676,
			Count:    5,
		},
		{
			Nodename: "spp-qa-vm-node-3",
			Usage:    0.45999999999999996,
			Rate:     3.725290298461914e-08,
			Capacity: 4290676,
			Count:    6,
		},
	}
	deployMap, err := GlobalPlan(context.TODO(), infos, 1, 19308043, 1)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, deployMap["spp-qa-vm-node-1"])
}
