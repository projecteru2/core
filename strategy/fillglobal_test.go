package strategy

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestFillGlobalPlan(t *testing.T) {
	// can fill
	strategyInfos := []Info{
		{
			Nodename: "n1",
			Capacity: 10,
			Count:    1,
		},
		{
			Nodename: "n2",
			Capacity: 10,
			Count:    0,
		},
	}
	deployMap, err := FillPlan(strategyInfos, 1, 2, 1, types.ResourceAll)
	assert.Nil(t, err)
	assert.EqualValues(t, 0, deployMap["n1"])
	assert.EqualValues(t, 1, deployMap["n2"])
	deployMap, err = FillGlobalPlan(strategyInfos, 1, 2, 1, types.ResourceAll)
	assert.Nil(t, err)
	assert.EqualValues(t, 0, deployMap["n1"])
	assert.EqualValues(t, 1, deployMap["n2"])

	// can't fill
	strategyInfos = []Info{
		{
			Nodename: "n1",
			Capacity: 10,
			Count:    1,
			Usages:   map[types.ResourceType]float64{types.ResourceAll: 0.1},
			Rates:    map[types.ResourceType]float64{types.ResourceAll: 0.1},
		},
	}
	_, err = FillPlan(strategyInfos, 1, 2, 1, types.ResourceAll)
	assert.EqualError(t, err, "Cannot alloc a fill node plan, each node has enough workloads")
	deployMap, err = FillGlobalPlan(strategyInfos, 1, 2, 1, types.ResourceAll)
	assert.Nil(t, err)
	assert.EqualValues(t, 1, deployMap["n1"])
}
