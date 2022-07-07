package calcium

import (
	"context"
	"errors"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/resources"
	resourcemocks "github.com/projecteru2/core/resources/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCalculateCapacity(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	engine := &enginemocks.API{}

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
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, errors.New("not implemented")).Times(3)
	_, err := c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// failed by get deploy status
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{"n1": 0}, nil)

	// failed by get deploy plan
	opts.DeployStrategy = "FAKE"
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// strategy: dummy
	opts.DeployStrategy = strategy.Dummy
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		map[string]*resources.NodeCapacityInfo{
			"n1": {
				NodeName: "n1",
				Capacity: 10,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   100,
			},
		},
		10, nil,
	)
	msg, err := c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, msg.NodeCapacities["n1"], 10)
	assert.Equal(t, msg.Total, 10)

	rmgr.AssertExpectations(t)
}
