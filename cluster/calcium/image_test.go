package calcium

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
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
	// fail by validating
	_, err := c.RemoveImage(ctx, &types.ImageOptions{Podname: ""})
	assert.Error(t, err)
	// fail by get nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// fail 0 nodes
	_, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	engine := &enginemocks.API{}
	nodes := []*types.Node{
		{
			NodeMeta: types.NodeMeta{
				Name: "test",
			},
			Engine: engine,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	// fail remove
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	ch, err := c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{"xx"}, nil)
	// sucess remove but prune fail
	engine.On("ImagesPrune", mock.Anything).Return(types.ErrBadStorage).Once()
	ch, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	for c := range ch {
		assert.True(t, c.Success)
	}
	engine.On("ImagesPrune", mock.Anything).Return(nil)
	ch, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	for c := range ch {
		assert.True(t, c.Success)
	}
}

func TestCacheImage(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	// fail by validating
	_, err := c.CacheImage(ctx, &types.ImageOptions{Podname: ""})
	assert.Error(t, err)
	// fail by get nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err = c.CacheImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// fail 0 nodes
	_, err = c.CacheImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	engine := &enginemocks.API{}
	nodes := []*types.Node{
		{
			NodeMeta: types.NodeMeta{
				Name: "test",
			},
			Engine: engine,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	// fail by ImageRemoteDigest
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", types.ErrNoETCD).Once()
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.CacheImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("yy", nil)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{"xx"}, nil)
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(ioutil.NopCloser(bytes.NewReader([]byte{})), nil)
	// succ
	ch, err = c.CacheImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	for c := range ch {
		assert.True(t, c.Success)
	}
}
