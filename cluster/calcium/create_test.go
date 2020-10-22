package calcium

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	"github.com/projecteru2/core/scheduler/resources"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateContainer(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.DeployOptions{}
	store := c.store.(*storemocks.Store)

	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// failed by GetPod
	store.On("GetPod", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	store.On("GetPod", mock.Anything, mock.Anything).Return(&types.Pod{Name: "test"}, nil)
	// failed by count
	opts.Count = 0
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	opts.Count = 1

	// failed by memory check
	opts.ResourceRequests = append(opts.ResourceRequests, resources.CPUMemResourceRequest{Memory: -1})
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	opts.ResourceRequests[0] = resources.CPUMemResourceRequest{Memory: 1}

	// failed by CPUQuota
	opts.ResourceRequests[0] = resources.CPUMemResourceRequest{CPUQuota: -1}
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
}

func TestCreateContainerTxn(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.DeployOptions{
		Count:            2,
		DeployStrategy:   strategy.Auto,
		Podname:          "p1",
		ResourceRequests: []types.ResourceRequest{resources.CPUMemResourceRequest{CPUQuota: 1}},
		Image:            "zc:test",
		Entrypoint:       &types.Entrypoint{},
	}
	store := &storemocks.Store{}
	sche := &schedulermocks.Scheduler{}
	scheduler.InitSchedulerV1(sche)
	c.store = store
	c.scheduler = sche
	engine := &enginemocks.API{}

	pod1 := &types.Pod{Name: "p1"}
	node1 := &types.Node{
		Name:   "n1",
		Engine: engine,
	}
	node2 := &types.Node{
		Name:   "n2",
		Engine: engine,
	}
	nodes := []*types.Node{node1, node2}

	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// doAllocResource fails: MakeDeployStatus
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
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
	sche.On("SelectMemoryNodes", mock.AnythingOfType("[]types.NodeInfo"), mock.AnythingOfType("float64"), mock.AnythingOfType("int64")).Return(
		func(nodesInfo []types.NodeInfo, _ float64, _ int64) []types.NodeInfo {
			for i := range nodesInfo {
				nodesInfo[i].Capacity = 1
			}
			return nodesInfo
		}, len(nodes), nil)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(
		errors.Wrap(context.DeadlineExceeded, "MakeDeployStatus"),
	).Once()
	ch, err := c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	cnt := 0
	for m := range ch {
		cnt++
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "MakeDeployStatus")
	}
	assert.EqualValues(t, 1, cnt)

	// commit resource changes fails: UpdateNodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(sis []types.StrategyInfo, need, total, _ int, resourceType types.ResourceType) (map[string]*types.DeployInfo, error) {
		deployInfos := make(map[string]*types.DeployInfo)
		for _, si := range sis {
			deployInfos[si.Nodename] = &types.DeployInfo{
				Deploy: 1,
			}
		}
		return deployInfos, nil
	}
	defer func() {
		strategy.Plans[strategy.Auto] = old
	}()
	store.On("UpdateNodes", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "UpdateNodes1")).Once()
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "UpdateNodes1")
	}
	assert.EqualValues(t, 1, cnt)
	assert.EqualValues(t, 1, node1.CPUUsed)
	assert.EqualValues(t, 1, node2.CPUUsed)
	node1.CPUUsed = 0
	node2.CPUUsed = 0

	// doCreateContainerOnNode fails: doGetAndPrepareNode
	store.On("UpdateNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
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
	store.On("GetNode",
		mock.AnythingOfType("*context.cancelCtx"),
		mock.AnythingOfType("string"),
	).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImageLocalDigest")).Twice()
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImagePull")).Twice()
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "ImagePull")
	}
	assert.EqualValues(t, 2, cnt)
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doDeployOneWorkload fails: VirtualizationCreate
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{""}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", nil)
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "VirtualizationCreate")).Twice()
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(nil)
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "VirtualizationCreate")
	}
	assert.EqualValues(t, 2, cnt)
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartContainer fails: AddContainer
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddContainer", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddContainer")).Twice()
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	for m := range ch {
		cnt++
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "AddContainer")
	}
	assert.EqualValues(t, 2, cnt)
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartContainer fails: first time AddContainer failed
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddContainer", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddContainer2")).Once()
	store.On("AddContainer", mock.Anything, mock.Anything).Return(nil).Once()
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	cnt = 0
	errCnt := 0
	for m := range ch {
		cnt++
		if m.Error != nil {
			assert.Error(t, m.Error)
			assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
			assert.Error(t, m.Error, "AddContainer2")
			errCnt++
		}
	}
	assert.EqualValues(t, 2, cnt)
	assert.EqualValues(t, 1, errCnt)
	assert.EqualValues(t, 1, node1.CPUUsed+node2.CPUUsed)
	return
}
