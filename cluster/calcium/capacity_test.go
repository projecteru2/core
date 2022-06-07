package calcium

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/resources"
	resourcemocks "github.com/projecteru2/core/resources/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func TestCalculateCapacity(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	engine := &enginemocks.API{}
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	// pod1 := &types.Pod{Name: "p1"}
	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "n1",
		},
		Engine: engine,
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{},
	}, nil)
	// failed by call plugin
	opts := &types.DeployOptions{
		Entrypoint: &types.Entrypoint{
			Name: "entry",
		},
		ResourceOpts:   types.WorkloadResourceOpts{},
		DeployStrategy: strategy.Auto,
		NodeFilter: types.NodeFilter{
			Includes: []string{"n1"},
		},
		Count: 3,
	}
	plugin.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("not implemented")).Once()
	_, err := c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// failed by get deploy status
	plugin.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodesDeployCapacityResponse{
		Nodes: map[string]*resources.NodeCapacityInfo{
			"n1": {
				NodeName: "n1",
				Capacity: 10,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   100,
			},
		},
		Total: 0,
	}, nil)
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{"n1": 0}, nil)

	// failed by get deploy plan
	opts.DeployStrategy = "FAKE"
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// strategy: auto
	opts.DeployStrategy = strategy.Auto
	msg, err := c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, msg.NodeCapacities["n1"], 3)
	assert.Equal(t, msg.Total, 3)

	// strategy: dummy
	opts.DeployStrategy = strategy.Dummy
	msg, err = c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, msg.NodeCapacities["n1"], 10)
	assert.Equal(t, msg.Total, 10)

	store.AssertExpectations(t)
}
