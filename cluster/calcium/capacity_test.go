package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func TestCalculateCapacity(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	engine := &enginemocks.API{}
	name := "n1"
	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: name,
		},
		Engine: engine,
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)

	opts := &types.DeployOptions{
		Entrypoint: &types.Entrypoint{
			Name: "entry",
		},
		Resources:      resourcetypes.Resources{},
		DeployStrategy: strategy.Auto,
		NodeFilter: &types.NodeFilter{
			Includes: []string{name},
		},
		Count: 3,
	}

	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, 0, types.ErrMockError,
	).Once()
	_, err := c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	nrim := map[string]*plugintypes.NodeDeployCapacity{
		name: {
			Capacity: 10,
			Usage:    0.5,
			Rate:     0.5,
			Weight:   100,
		},
	}
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nrim, 100, nil,
	).Times(3)
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{name: 0}, nil)
	opts.Count = -1
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	opts.Count = 1
	_, err = c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)

	opts.DeployStrategy = strategy.Dummy

	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrMockError).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, -1, nil).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nrim, 10, nil,
	)
	msg, err := c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, msg.NodeCapacities[name], 10)
	assert.Equal(t, msg.Total, 10)

	rmgr.AssertExpectations(t)
}
