package calcium

import (
	"context"
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

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
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
		ResourceOpts:   types.WorkloadResourceOpts{},
		DeployStrategy: strategy.Auto,
		NodeFilter: types.NodeFilter{
			Includes: []string{name},
		},
		Count: 3,
	}

	// failed by call plugin
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrNoETCD).Once()
	_, err := c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// failed by get deploy status
	nrim := map[string]*resources.NodeCapacityInfo{
		name: {
			NodeName: name,
			Capacity: 10,
			Usage:    0.5,
			Rate:     0.5,
			Weight:   100,
		},
	}
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nrim, 100, nil).Times(3)
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// failed by get deploy strategy
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{name: 0}, nil)
	opts.Count = -1
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// success
	opts.Count = 1
	_, err = c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)

	// strategy: dummy
	opts.DeployStrategy = strategy.Dummy

	// failed by GetNodesDeployCapacity
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrNoETCD).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// failed by total <= 0
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, -1, nil).Once()
	_, err = c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)

	// success
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nrim, 10, nil)
	msg, err := c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, msg.NodeCapacities[name], 10)
	assert.Equal(t, msg.Total, 10)

	rmgr.AssertExpectations(t)
}
