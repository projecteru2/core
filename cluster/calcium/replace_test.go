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

func TestReplaceWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	_, err := c.ReplaceWorkload(ctx, &types.ReplaceOptions{
		DeployOptions: types.DeployOptions{
			Entrypoint: &types.Entrypoint{
				Name: "bad_entrypoint_name",
			},
		},
	})
	assert.Error(t, err)

	opts := &types.ReplaceOptions{
		DeployOptions: types.DeployOptions{
			Name:  "appname",
			Image: "image:latest",
			Entrypoint: &types.Entrypoint{
				Name: "nice-entry-name",
			},
		},
	}

	workload := &types.Workload{
		ID:       "xx",
		Name:     "yy",
		Nodename: "testnode",
	}
	// failed by ListWorkload
	store.On("ListWorkloads", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.ReplaceWorkload(ctx, opts)
	assert.Error(t, err)
	store.On("ListWorkloads", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	// failed by withWorkloadLocked
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil).Twice()
	// ignore because pod not fit
	opts.Podname = "wtf"
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for range ch {
	}
	workload.Podname = "wtf"
	opts.NetworkInherit = true
	// failed by inspect
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	// failed by not running
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{Running: false}, nil).Once()
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{Running: true}, nil)
	// failed by not fit
	opts.FilterLabels = map[string]string{"x": "y"}
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	// failed by get node
	opts.FilterLabels = map[string]string{}
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
		},
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil).Once()
	// failed by no image
	opts.Image = ""
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	opts.Image = "image:latest"
	node.Engine = engine
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{"id"}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("id", nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by VirtualizationCopyFrom
	opts.Copy = map[string]string{"src": "dst"}
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(nil, "", types.ErrBadWorkloadID).Once()
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.False(t, r.Remove.Success)
	}
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(ioutil.NopCloser(bytes.NewReader([]byte{})), "", nil)
	opts.DeployOptions.Data = map[string]types.ReaderManager{}
	// failed by Stop
	engine.On("VirtualizationStop", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	ch, err = c.ReplaceWorkload(ctx, opts)
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
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil).Once()
	ch, err = c.ReplaceWorkload(ctx, opts)
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
	store.On("AddWorkload", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// failed by remove workload
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.NotNil(t, r.Create)
		assert.False(t, r.Remove.Success)
		assert.Nil(t, r.Create.Error)
	}
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	// succ
	ch, err = c.ReplaceWorkload(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
		assert.NotNil(t, r.Remove)
		assert.NotNil(t, r.Create)
		assert.True(t, r.Remove.Success)
		assert.Nil(t, r.Create.Error)
	}
}
