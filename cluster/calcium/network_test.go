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
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return([]*types.Node{}, nil).Once()
	c.store = store

	// No nodes
	_, err := c.ListNetworks(ctx, "", "")
	assert.Error(t, err)
	// vaild
	engine := &enginemocks.API{}
	node := &types.Node{
		Name:      "test",
		Available: true,
		Engine:    engine,
	}
	name := "test"
	engine.On("NetworkList", mock.Anything, mock.Anything).Return([]*enginetypes.Network{{Name: name}}, nil)
	store.On("GetNodesByPod", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return([]*types.Node{node}, nil)
	ns, err := c.ListNetworks(ctx, "", "")
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 1)
	assert.Equal(t, ns[0].Name, name)
}
