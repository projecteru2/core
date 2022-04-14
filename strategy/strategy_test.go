package strategy

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func deployedNodes() []Info {
	return []Info{
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

func TestDeploy(t *testing.T) {
	opts := &types.DeployOptions{
		DeployStrategy: "invalid",
		Count:          1,
		NodesLimit:     3,
	}
	_, err := Deploy(context.TODO(), opts.DeployStrategy, opts.Count, opts.NodesLimit, nil, 2)
	opts.DeployStrategy = "AUTO"
	Plans["test"] = func(_ context.Context, _ []Info, _, _, _ int) (map[string]int, error) {
		return nil, nil
	}
	_, err = Deploy(context.TODO(), opts.DeployStrategy, opts.Count, opts.NodesLimit, nil, 2)
	assert.Error(t, err)
}
