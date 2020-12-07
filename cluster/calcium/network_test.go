package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestNetwork(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.ListNetworks(ctx, "", "")
	assert.Error(t, err)
	// No nodes
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	_, err = c.ListNetworks(ctx, "", "")
	assert.Error(t, err)
	// vaild
	engine := &enginemocks.API{}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
		},
		Available: true,
		Engine:    engine,
	}
	name := "test"
	engine.On("NetworkList", mock.Anything, mock.Anything).Return([]*enginetypes.Network{{Name: name}}, nil)
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	ns, err := c.ListNetworks(ctx, "", "xx")
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 1)
	assert.Equal(t, ns[0].Name, name)
}

func TestConnectNetwork(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	engine := &enginemocks.API{}
	workload := &types.Workload{Engine: engine}

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrBadMeta).Once()
	_, err := c.ConnectNetwork(ctx, "network", "123", "", "")
	assert.Error(t, err)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	engine.On("NetworkConnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{}, nil)
	_, err = c.ConnectNetwork(ctx, "network", "123", "", "")
	assert.NoError(t, err)
}

func TestDisConnectNetwork(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	engine := &enginemocks.API{}
	workload := &types.Workload{Engine: engine}

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrBadMeta).Once()
	err := c.DisconnectNetwork(ctx, "network", "123", true)
	assert.Error(t, err)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	engine.On("NetworkDisconnect", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	err = c.DisconnectNetwork(ctx, "network", "123", true)
	assert.NoError(t, err)
}
