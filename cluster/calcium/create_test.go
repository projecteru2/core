package calcium

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestCreateWorkloadValidating(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:    "deployname",
		Podname: "somepod",
		Image:   "image:todeploy",
		Count:   1,
		Entrypoint: &types.Entrypoint{
			Name: "some-nice-entrypoint",
		},
		NodeFilter: &types.NodeFilter{},
	}
	// failed by validating
	opts.Name = ""
	_, err := c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Name = "deployname"

	opts.Podname = ""
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Podname = "somepod"

	opts.Image = ""
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Image = "image:todeploy"

	opts.Count = 0
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Count = 1

	opts.Entrypoint.Name = "bad_entry_name"
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Entrypoint.Name = "some-nice-entrypoint"
}

func TestCreateWorkloadTxn(t *testing.T) {
	c, nodes := newCreateWorkloadCluster(t)
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:           "zc:name",
		Count:          2,
		DeployStrategy: strategy.Auto,
		Podname:        "p1",
		Resources:      types.Resources{},
		Image:          "zc:test",
		Entrypoint: &types.Entrypoint{
			Name: "good-entrypoint",
		},
		NodeFilter: &types.NodeFilter{},
	}

	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	mwal := &walmocks.WAL{}
	c.wal = mwal
	var walCommitted bool
	commit := wal.Commit(func() error {
		walCommitted = true
		return nil
	})
	mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)
	node1, node2 := nodes[0], nodes[1]

	// doAllocResource fails: GetNodesDeployCapacity
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, 0, types.ErrMockError,
	).Once()
	ch, err := c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt := 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error, "key is empty")
	}
	assert.EqualValues(t, 1, cnt)
	assert.True(t, walCommitted)
	walCommitted = false
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		map[string]*plugintypes.NodeDeployCapacity{
			node1.Name: {
				Capacity: 10,
				Usage:    0.5,
				Rate:     0.05,
				Weight:   100,
			},
			node2.Name: {
				Capacity: 10,
				Usage:    0.5,
				Rate:     0.05,
				Weight:   100,
			},
		}, 20, nil,
	)

	// doAllocResource fails: GetDeployStatus
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "GetDeployStatus")).Once()
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.ErrorIs(t, m.Error, context.DeadlineExceeded)
		assert.Error(t, m.Error, "GetDeployStatus")
	}
	assert.EqualValues(t, 1, cnt)
	assert.True(t, walCommitted)
	walCommitted = false
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{}, nil)

	// doAllocResource fails: Alloc
	rmgr.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil, nil, types.ErrMockError,
	).Once()
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error, "DeadlineExceeded")
	}
	assert.EqualValues(t, 1, cnt)
	assert.True(t, walCommitted)
	walCommitted = false
	rmgr.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		[]types.Resources{{}, {}},
		[]types.Resources{
			{node1.Name: {}},
			{node2.Name: {}},
		},
		nil,
	)
	rmgr.On("RollbackAlloc", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("GetNode",
		mock.AnythingOfType("*context.timerCtx"),
		mock.AnythingOfType("string"),
	).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)
	engine := node1.Engine.(*enginemocks.API)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImageLocalDigest")).Twice()
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImagePull")).Twice()
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error, "ImagePull")
	}
	assert.EqualValues(t, 2, cnt)
	assert.True(t, walCommitted)

	// doDeployOneWorkload fails: VirtualizationCreate
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{""}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", nil)
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "VirtualizationCreate")).Twice()
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	walCommitted = false
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "VirtualizationCreate")
	}
	assert.EqualValues(t, 2, cnt)
	assert.True(t, walCommitted)

	// doCreateAndStartWorkload fails: AddWorkload
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddWorkload")).Twice()
	walCommitted = false
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "AddWorkload")
	}
	assert.EqualValues(t, 2, cnt)
	assert.True(t, walCommitted)

	// doCreateAndStartWorkload fails: first time AddWorkload failed
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddWorkload2")).Once()
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	walCommitted = false
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	errCnt := 0
	for m := range ch {
		cnt++
		if m.Error != nil {
			assert.Error(t, m.Error)
			assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
			assert.Error(t, m.Error, "AddWorkload2")
			errCnt++
		}
	}
	assert.EqualValues(t, 2, cnt)
	assert.EqualValues(t, 1, errCnt)
	assert.True(t, walCommitted)
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func newCreateWorkloadCluster(t *testing.T) (*Calcium, []*types.Node) {
	c := NewTestCluster()

	engine := &enginemocks.API{}
	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "n1",
		},
		Engine: engine,
	}
	node2 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "n2",
		},
		Engine: engine,
	}
	nodes := []*types.Node{node1, node2}

	// for processing
	store := c.store.(*storemocks.Store)
	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// for lock
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	// for get node
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)

	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)

	return c, nodes
}
