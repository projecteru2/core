package strategy

import (
	"context"
	"testing"

	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	resourcetypesmocks "github.com/projecteru2/core/resources/types/mocks"
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
	_, err := Deploy(context.TODO(), opts, nil, 2)
	opts.DeployStrategy = "AUTO"
	Plans["test"] = func(_ context.Context, _ []Info, _, _, _ int) (map[string]int, error) {
		return nil, nil
	}
	_, err = Deploy(context.TODO(), opts, nil, 2)
	assert.Error(t, err)
}

func TestNewInfos(t *testing.T) {
	rrs, err := resources.MakeRequests(types.ResourceOptions{})
	assert.Nil(t, err)
	nodeMap := map[string]*types.Node{
		"node1": {},
		"node2": {},
	}
	mockPlan := &resourcetypesmocks.ResourcePlans{}
	mockPlan.On("Capacity").Return(map[string]int{"node1": 1})
	plans := []resourcetypes.ResourcePlans{mockPlan}
	NewInfos(rrs, nodeMap, plans)
}
