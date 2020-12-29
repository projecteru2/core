package calcium

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
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

	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
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

	// failed by memory check
	opts.ResourceOpts = types.ResourceOptions{MemoryLimit: -1}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	ch, err := c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	for m := range ch {
		assert.Error(t, m.Error)
	}

	// failed by CPUQuota
	opts.ResourceOpts = types.ResourceOptions{CPUQuotaLimit: -1, MemoryLimit: 1}
	ch, err = c.CreateWorkload(ctx, opts)
	assert.Nil(t, err)
	for m := range ch {
		assert.Error(t, m.Error)
	}
}

func TestCreateWorkloadTxn(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:           "zc:name",
		Count:          2,
		DeployStrategy: strategy.Auto,
		Podname:        "p1",
		ResourceOpts:   types.ResourceOptions{CPUQuotaLimit: 1},
		Image:          "zc:test",
		Entrypoint: &types.Entrypoint{
			Name: "good-entrypoint",
		},
	}
	store := &storemocks.Store{}
	sche := &schedulermocks.Scheduler{}
	scheduler.InitSchedulerV1(sche)
	c.store = store
	c.scheduler = sche
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

	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
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
	sche.On("SelectStorageNodes", mock.AnythingOfType("[]resourcetypes.ScheduleInfo"), mock.AnythingOfType("int64")).Return(func(scheduleInfos []resourcetypes.ScheduleInfo, _ int64) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, len(nodes), nil)
	sche.On("SelectStorageNodes", mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("int64")).Return(func(scheduleInfos []resourcetypes.ScheduleInfo, _ int64) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, len(nodes), nil)
	sche.On("SelectVolumeNodes", mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("types.VolumeBindings")).Return(func(scheduleInfos []resourcetypes.ScheduleInfo, _ types.VolumeBindings) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, nil, len(nodes), nil)
	sche.On("SelectMemoryNodes", mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("float64"), mock.AnythingOfType("int64")).Return(
		func(scheduleInfos []resourcetypes.ScheduleInfo, _ float64, _ int64) []resourcetypes.ScheduleInfo {
			for i := range scheduleInfos {
				scheduleInfos[i].Capacity = 1
			}
			return scheduleInfos
		}, len(nodes), nil)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(
		errors.Wrap(context.DeadlineExceeded, "MakeDeployStatus"),
	).Once()
	ch, err := c.CreateWorkload(ctx, opts)
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
	strategy.Plans[strategy.Auto] = func(sis []strategy.Info, need, total, _ int, resourceType types.ResourceType) (map[string]int, error) {
		deployInfos := make(map[string]int)
		for _, si := range sis {
			deployInfos[si.Nodename] = 1
		}
		return deployInfos, nil
	}
	defer func() {
		strategy.Plans[strategy.Auto] = old
	}()
	store.On("UpdateNodes", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "UpdateNodes1")).Once()
	ch, err = c.CreateWorkload(ctx, opts)
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

	// doCreateWorkloadOnNode fails: doGetAndPrepareNode
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
	ch, err = c.CreateWorkload(ctx, opts)
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
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
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
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartWorkload fails: AddWorkload
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddWorkload")).Twice()
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
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartWorkload fails: first time AddWorkload failed
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddWorkload2")).Once()
	store.On("AddWorkload", mock.Anything, mock.Anything).Return(nil).Once()
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
	assert.EqualValues(t, 1, node1.CPUUsed+node2.CPUUsed)
	return
}
