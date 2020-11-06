package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRemoveContainer(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	// failed by GetContainer
	store.On("GetContainers", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.RemoveContainer(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	container := &types.Container{
		ID:       "xx",
		Name:     "test",
		Nodename: "test",
	}
	store.On("GetContainers", mock.Anything, mock.Anything).Return([]*types.Container{container}, nil)
	// failed by GetNode
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err = c.RemoveContainer(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	node := &types.Node{
		Name: "test",
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by Remove
	ch, err = c.RemoveContainer(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	assert.NoError(t, c.doRemoveContainerSync(ctx, []string{"xx"}))
	engine := &enginemocks.API{}
	container.Engine = engine
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("GetContainers", mock.Anything, mock.Anything).Return([]*types.Container{container}, nil)
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// success
	ch, err = c.RemoveContainer(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.True(t, r.Success)
	}
}
