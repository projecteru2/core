package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/resources"
	resourcemocks "github.com/projecteru2/core/resources/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateWorkload(t *testing.T) {
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
	}
	store := c.store.(*storemocks.Store)

	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

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
		ResourceOpts:   types.WorkloadResourceOpts{},
		Image:          "zc:test",
		Entrypoint: &types.Entrypoint{
			Name: "good-entrypoint",
		},
	}

	store := &storemocks.Store{}
	c.store = store
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	node1, node2 := nodes[0], nodes[1]

	c.wal = &WAL{WAL: &walmocks.WAL{}}
	mwal := c.wal.WAL.(*walmocks.WAL)
	var walCommitted bool
	commit := wal.Commit(func() error {
		walCommitted = true
		return nil
	})
	mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)

	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// doAllocResource fails: GetNodesDeployCapacity
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("GetNode",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
	).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{},
	}, nil)
	plugin.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("GetNodesDeployCapacity")).Once()

	ch, err := c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	cnt := 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error, "GetNodesDeployCapacity")
	}
	assert.EqualValues(t, 1, cnt)
	assert.True(t, walCommitted)
	walCommitted = false

	// doAllocResource fails: GetDeployStatus
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "GetDeployStatus")).Once()
	plugin.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodesDeployCapacityResponse{
		Nodes: map[string]*resources.NodeCapacityInfo{
			node1.Name: {
				NodeName: node1.Name,
				Capacity: 10,
				Usage:    0.5,
				Rate:     0.05,
				Weight:   100,
			},
			node2.Name: {
				NodeName: node2.Name,
				Capacity: 10,
				Usage:    0.5,
				Rate:     0.05,
				Weight:   100,
			},
		},
		Total: 20,
	}, nil)

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

	// doAllocResource fails: Alloc
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{}, nil)
	plugin.On("GetDeployArgs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Once()
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

	// doCreateWorkloadOnNode fails: doGetAndPrepareNode
	plugin.On("GetDeployArgs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetDeployArgsResponse{
		EngineArgs:   []types.EngineArgs{},
		ResourceArgs: []types.WorkloadResourceArgs{},
	}, nil)
	plugin.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&resources.SetNodeResourceUsageResponse{
		Before: types.NodeResourceArgs{},
		After:  types.NodeResourceArgs{},
	}, nil)
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
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD)
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
	c.wal = &WAL{WAL: &walmocks.WAL{}}
	c.store = &storemocks.Store{}
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	engine := &enginemocks.API{}
	pod1 := &types.Pod{Name: "p1"}
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

	store := c.store.(*storemocks.Store)
	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// doAllocResource fails: MakeDeployStatus
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("GetNode",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
	).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)

	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{},
	}, nil)

	mwal := c.wal.WAL.(*walmocks.WAL)
	commit := wal.Commit(func() error { return nil })
	mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)

	return c, nodes
}
