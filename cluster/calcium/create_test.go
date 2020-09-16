package calcium

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	st "github.com/projecteru2/core/store"
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
	opts.Memory = -1
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	opts.Memory = 1

	// failed by CPUQuota
	opts.CPUQuota = -1
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
}

func TestCreateContainerTxn(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.DeployOptions{
		Count:          2,
		DeployStrategy: strategy.Auto,
		CPUQuota:       1,
		Image:          "zc:test",
		Entrypoint:     &types.Entrypoint{},
	}
	store := &storemocks.Store{}
	scheduler := &schedulermocks.Scheduler{}
	c.store = store
	c.scheduler = scheduler
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

	// GetPod fails
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil,
		errors.Wrap(context.DeadlineExceeded, "GetNodesByPod"),
	).Once()
	_, err := c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Error(t, err, "GetNodesByPod")

	// doAllocResource fails: MakeDeployStatus
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
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
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(
		nil,
		errors.Wrap(context.DeadlineExceeded, "MakeDeployStatus"),
	).Once()
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Error(t, err, "MakeDeployStatus")

	// doAllocResource fails: UpdateNodeResource for 1st node
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("MakeDeployStatus", ctx, opts, mock.AnythingOfType("[]types.NodeInfo")).Return(
		func(_ context.Context, _ *types.DeployOptions, nodesInfo []types.NodeInfo) []types.NodeInfo {
			return nodesInfo
		}, nil)
	scheduler.On("SelectMemoryNodes", mock.AnythingOfType("[]types.NodeInfo"), mock.AnythingOfType("float64"), mock.AnythingOfType("int64")).Return(
		func(nodesInfo []types.NodeInfo, _ float64, _ int64) []types.NodeInfo {
			return nodesInfo
		}, len(nodes), nil)
	scheduler.On("SelectStorageNodes", mock.AnythingOfType("[]types.NodeInfo"), mock.AnythingOfType("int64")).Return(
		func(nodesInfo []types.NodeInfo, _ int64) []types.NodeInfo {
			return nodesInfo
		},
		len(nodes), nil,
	)
	scheduler.On("SelectVolumeNodes", mock.AnythingOfType("[]types.NodeInfo"), mock.AnythingOfType("types.VolumeBindings")).Return(
		func(nodesInfo []types.NodeInfo, _ types.VolumeBindings) []types.NodeInfo {
			return nodesInfo
		},
		nil, len(nodes), nil,
	)
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(nodesInfo []types.NodeInfo, need, total, _ int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
		for i := range nodesInfo {
			nodesInfo[i].Deploy = 1
		}
		return nodesInfo, nil
	}
	defer func() {
		strategy.Plans[strategy.Auto] = old
	}()
	store.On("UpdateNodeResource",
		mock.AnythingOfType("*context.timerCtx"),
		mock.AnythingOfType("*types.Node"),
		mock.AnythingOfType("types.ResourceMap"),
		mock.AnythingOfType("float64"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("types.ResourceMap"),
		mock.AnythingOfType("string")).Return(
		func(ctx context.Context, node *types.Node, _ types.CPUMap, quota float64, _, _ int64, _ types.VolumeMap, action string) error {
			if action == st.ActionDecr {
				return errors.Wrap(context.DeadlineExceeded, "UpdateNodeResource")
			}
			if action == st.ActionIncr {
				quota = -quota
			}
			node.CPUUsed += quota
			return nil
		},
	).Once()
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Error(t, err, "UpdateNodeResource")
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doAllocResource fails: UpdateNodeResource for 2nd node
	cnt := 0
	store.On("UpdateNodeResource",
		mock.AnythingOfType("*context.timerCtx"),
		mock.AnythingOfType("*types.Node"),
		mock.AnythingOfType("types.ResourceMap"),
		mock.AnythingOfType("float64"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("types.ResourceMap"),
		mock.AnythingOfType("string")).Return(
		func(ctx context.Context, node *types.Node, _ types.CPUMap, quota float64, _, _ int64, _ types.VolumeMap, action string) error {
			if action == st.ActionDecr {
				cnt++
				if cnt == 2 {
					return errors.Wrap(context.DeadlineExceeded, "UpdateNodeResource2")
				}
			}
			if action == st.ActionIncr {
				quota = -quota
			}
			node.CPUUsed += quota
			return nil
		},
	).Times(3)
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Error(t, err, "UpdateNodeResource2")
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)
	assert.EqualValues(t, 2, cnt)

	// doAllocResource fails: SaveProcessing
	store.On("UpdateNodeResource",
		mock.AnythingOfType("*context.timerCtx"),
		mock.AnythingOfType("*types.Node"),
		mock.AnythingOfType("types.ResourceMap"),
		mock.AnythingOfType("float64"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("int64"),
		mock.AnythingOfType("types.ResourceMap"),
		mock.AnythingOfType("string")).Return(
		func(ctx context.Context, node *types.Node, _ types.CPUMap, quota float64, _, _ int64, _ types.VolumeMap, action string) error {
			if action == st.ActionIncr {
				quota = -quota
			}
			node.CPUUsed += quota
			return nil
		},
	)
	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "SaveProcessing")).Once()
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded))
	assert.Error(t, err, "SaveProcessing")
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateContainerOnNode fails: doGetAndPrepareNode
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
	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImageLocalDigest")).Twice()
	engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImagePull")).Twice()
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err := c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	for m := range ch {
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "ImagePull")
	}
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartContainer fails: VirtualizationCreate
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{""}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", nil)
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "VirtualizationCreate")).Twice()
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	for m := range ch {
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "VirtualizationCreate")
	}
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartContainer fails: AddContainer
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddContainer", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddContainer")).Twice()
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(nil).Twice()
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	for m := range ch {
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "AddContainer")
	}
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)

	// doCreateAndStartContainer fails: RemoveContainer
	store.On("AddContainer", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddContainer")).Twice()
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "RemoveContainer")).Twice()
	ch, err = c.CreateContainer(ctx, opts)
	assert.Nil(t, err)
	for m := range ch {
		assert.Error(t, m.Error)
		assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
		assert.Error(t, m.Error, "AddContainer")
	}
	assert.EqualValues(t, 0, node1.CPUUsed)
	assert.EqualValues(t, 0, node2.CPUUsed)
}
