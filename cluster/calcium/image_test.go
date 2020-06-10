package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRemoveImage(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	// fail by get nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err := c.RemoveImage(ctx, "", "", []string{}, 0, false)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// fail 0 nodes
	_, err = c.RemoveImage(ctx, "", "", []string{}, 0, false)
	assert.Error(t, err)
	engine := &enginemocks.API{}
	nodes := []*types.Node{
		{
			Name:   "test",
			Engine: engine,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	// fail remove
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	ch, err := c.RemoveImage(ctx, "", "", []string{"xx"}, 0, false)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{"xx"}, nil)
	// sucess remove but prune fail
	engine.On("ImagesPrune", mock.Anything).Return(types.ErrBadStorage).Once()
	ch, err = c.RemoveImage(ctx, "", "", []string{"xx"}, 0, true)
	for c := range ch {
		assert.True(t, c.Success)
	}
	engine.On("ImagesPrune", mock.Anything).Return(nil)
	ch, err = c.RemoveImage(ctx, "", "", []string{"xx"}, 0, true)
	for c := range ch {
		assert.True(t, c.Success)
	}
}

func TestCacheImage(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	// fail by get nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err := c.CacheImage(ctx, "", "", []string{}, 0)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// fail 0 nodes
	_, err = c.CacheImage(ctx, "", "", []string{}, 0)
	assert.Error(t, err)
	engine := &enginemocks.API{}
	nodes := []*types.Node{
		{
			Name:   "test",
			Engine: engine,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	// fail by ImageRemoteDigest
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", types.ErrNoETCD).Once()
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.CacheImage(ctx, "", "", []string{"xx"}, 0)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("yy", nil)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{"xx"}, nil)
	imageCh := make(chan *enginetypes.ImageMessage)
	close(imageCh)
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(imageCh, nil)
	// succ
	ch, err = c.CacheImage(ctx, "", "", []string{"xx"}, 0)
	for c := range ch {
		assert.True(t, c.Success)
	}
}
