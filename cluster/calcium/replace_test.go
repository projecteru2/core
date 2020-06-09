package calcium

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestReplaceContainer(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	opts := &types.ReplaceOptions{
		DeployOptions: types.DeployOptions{
			Entrypoint: &types.Entrypoint{},
		},
	}

	container := &types.Container{
		ID:   "xx",
		Name: "yy",
	}
	// failed by ListContainer
	store.On("ListContainers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.ReplaceContainer(ctx, opts)
	assert.Error(t, err)
	store.On("ListContainers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Container{container}, nil)
	// failed by withContainerLocked
	store.On("GetContainers", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("GetContainers", mock.Anything, mock.Anything).Return([]*types.Container{container}, nil).Twice()
	// ignore because pod not fit
	opts.Podname = "wtf"
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for range ch {
	}
	container.Podname = "wtf"
	opts.NetworkInherit = true
	// failed by inspect
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine := &enginemocks.API{}
	container.Engine = engine
	store.On("GetContainers", mock.Anything, mock.Anything).Return([]*types.Container{container}, nil)
	// failed by not running
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{Running: false}, nil).Once()
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{Running: true}, nil)
	// failed by not fit
	opts.FilterLabels = map[string]string{"x": "y"}
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	// failed by get node
	opts.FilterLabels = map[string]string{}
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	node := &types.Node{
		Name: "test",
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil).Once()
	// failed by no image
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	node.Engine = engine
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{"id"}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("id", nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by VirtualizationCopyFrom
	opts.Image = "xx"
	opts.Copy = map[string]string{"src": "dst"}
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(nil, "", types.ErrBadContainerID).Once()
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(ioutil.NopCloser(bytes.NewReader([]byte{})), "", nil)
	opts.DeployOptions.Data = map[string]*bytes.Reader{}
	// failed by Stop
	engine.On("VirtualizationStop", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	engine.On("VirtualizationStop", mock.Anything, mock.Anything).Return(nil)
	// failed by VirtualizationCreate
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, types.ErrCannotGetEngine).Once()
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "new"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationCopyTo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{User: "test"}, nil)
	store.On("AddContainer", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// failed by remove container
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.NotNil(t, r.Create)
		assert.False(t, r.Remove.Success)
		assert.Nil(t, r.Create.Error)
	}
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(nil)
	// succ
	ch, err = c.ReplaceContainer(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.NotNil(t, r.Create)
		assert.True(t, r.Remove.Success)
		assert.Nil(t, r.Create.Error)
	}
}
