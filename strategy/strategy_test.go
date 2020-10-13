package strategy

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func deployedNodes() []types.StrategyInfo {
	return []types.StrategyInfo{
		{
			Nodename: "n1",
			Capacity: 10,
			Count:    2,
		},
		{
			Nodename: "n2",
			Capacity: 10,
			Count:    3,
		},
		{
			Nodename: "n3",
			Capacity: 10,
			Count:    5,
		},
		{
			Nodename: "n4",
			Capacity: 10,
			Count:    7,
		},
	}
}

func TestScoreSort(t *testing.T) {
	ns := []types.StrategyInfo{
		{
			Nodename: "n1",
			Usages: map[types.ResourceType]float64{
				types.ResourceCPU:    0.1,
				types.ResourceVolume: 0.3,
				types.ResourceMemory: 0.4,
			},
		},
		{
			Nodename: "n2",
			Usages: map[types.ResourceType]float64{
				types.ResourceCPU:    0.3,
				types.ResourceVolume: 0.3,
				types.ResourceMemory: 0.1,
			},
		},
		{
			Nodename: "n3",
			Usages: map[types.ResourceType]float64{
				types.ResourceCPU:    0.2,
				types.ResourceVolume: 0.3,
				types.ResourceMemory: 0.1,
			},
		},
	}

	scoreSort(ns, types.ResourceCPU)
	assert.Equal(t, ns[0].Nodename, "n1")
	assert.Equal(t, ns[1].Nodename, "n3")
	assert.Equal(t, ns[2].Nodename, "n2")

	scoreSort(ns, types.ResourceCPU|types.ResourceMemory)
	assert.Equal(t, ns[0].Nodename, "n3")
	assert.Equal(t, ns[1].Nodename, "n2")
	assert.Equal(t, ns[2].Nodename, "n1")
}

func TestDeploy(t *testing.T) {
	opts := &types.DeployOptions{
		DeployStrategy: "invalid",
		Count:          1,
		NodesLimit:     3,
	}
	_, err := Deploy(opts, nil, 2, types.ResourceCPU)
	opts.DeployStrategy = "AUTO"
	Plans["test"] = func(_ []types.StrategyInfo, _, _, _ int, _ types.ResourceType) (map[string]*types.DeployInfo, error) {
		return nil, nil
	}
	_, err = Deploy(opts, nil, 2, types.ResourceCPU)
	assert.Nil(t, err)
}
