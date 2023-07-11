package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestListNetworks(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	store := c.store.(*storemocks.Store)

	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.ListNetworks(ctx, "", "")
	assert.Error(t, err)

	// No nodes
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	_, err = c.ListNetworks(ctx, "", "")
	assert.Error(t, err)

	// vaild
	name := "test"
	engine := &enginemocks.API{}
	engine.On("NetworkList", mock.Anything, mock.Anything).Return([]*enginetypes.Network{{Name: name}}, nil)
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: name,
		},
		Available: true,
		Engine:    engine,
	}
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	ns, err := c.ListNetworks(ctx, "", "xx")
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 1)
	assert.Equal(t, ns[0].Name, name)
	store.AssertExpectations(t)
}

func TestConnectNetwork(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	store := c.store.(*storemocks.Store)

	// failed by GetWorkload
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.ConnectNetwork(ctx, "network", "123", "", "")
	assert.Error(t, err)

	// success
	engine := &enginemocks.API{}
	engine.On("NetworkConnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
	workload := &types.Workload{Engine: engine}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	_, err = c.ConnectNetwork(ctx, "network", "123", "", "")
	assert.NoError(t, err)
	store.AssertExpectations(t)
}

func TestDisConnectNetwork(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	store := c.store.(*storemocks.Store)

	// failed by GetWorkload
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	assert.Error(t, c.DisconnectNetwork(ctx, "network", "123", true))

	// success
	engine := &enginemocks.API{}
	engine.On("NetworkDisconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workload := &types.Workload{Engine: engine}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	assert.NoError(t, c.DisconnectNetwork(ctx, "network", "123", true))
	store.AssertExpectations(t)
}
