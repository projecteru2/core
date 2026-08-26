package calcium

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestCacheImage(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	_, err := c.CacheImage(ctx, &types.ImageOptions{Podname: ""})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.CacheImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", types.ErrMockError).Once()
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.CacheImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("yy", nil)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{"xx"}, nil)
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(io.NopCloser(bytes.NewReader([]byte{})), nil)
	ch, err = c.CacheImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	assert.NoError(t, err)
	for c := range ch {
		assert.True(t, c.Success)
	}
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func TestRemoveImage(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	_, err := c.RemoveImage(ctx, &types.ImageOptions{Podname: ""})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}})
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ImageRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]string{"xx"}, nil)
	engine.On("ImagesPrune", mock.Anything).Return(types.ErrMockError).Once()
	ch, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}, Prune: true})
	assert.NoError(t, err)
	for c := range ch {
		assert.True(t, c.Success)
	}
	engine.On("ImagesPrune", mock.Anything).Return(nil)
	ch, err = c.RemoveImage(ctx, &types.ImageOptions{Podname: "podname", Images: []string{"xx"}, Prune: true})
	assert.NoError(t, err)
	for c := range ch {
		assert.True(t, c.Success)
	}
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func TestListImage(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	_, err := c.ListImage(ctx, &types.ImageOptions{})
	assert.ErrorIs(t, err, types.ErrEmptyPodName)

	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.ListImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	_, err = c.ListImage(ctx, &types.ImageOptions{Podname: "podname"})
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	engine.On("ImageList", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.ListImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.NoError(t, err)
	msg := <-ch
	assert.Error(t, msg.Error)
	engine.On("ImageList", mock.Anything, mock.Anything).Return(
		[]*enginetypes.Image{{ID: "123"}}, nil,
	)
	ch, err = c.ListImage(ctx, &types.ImageOptions{Podname: "podname"})
	assert.NoError(t, err)
	msg = <-ch
	assert.Equal(t, msg.Images[0].ID, "123")
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
}
