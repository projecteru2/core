package calcium

import (
	"context"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"

	enginetypes "github.com/docker/docker/api/types"
	enginemocks "github.com/projecteru2/core/3rdmocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/scheduler"

	"github.com/stretchr/testify/assert"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestReallocMem(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	store := &storemocks.Store{}
	store.On("GetContainers", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(nil, types.ErrNoBuildPod).Once()
	c.store = store

	_, err := c.ReallocResource(ctx, []string{}, 0.0, 0)
	assert.Error(t, err)

	engine := &enginemocks.APIClient{}
	engine.On("ContainerInspect", mock.Anything, mock.Anything).Return(enginetypes.ContainerJSON{}, nil)

	node1 := &types.Node{
		Name:   "node1",
		MemCap: types.GByte,
		CPU:    types.CPUMap{"0": 50, "1": 70},
		Engine: engine,
	}
	node2 := &types.Node{
		Name:   "node2",
		MemCap: types.GByte,
		CPU:    types.CPUMap{"0": 50, "1": 70},
		Engine: engine,
	}

	pod1 := &types.Pod{
		Name:  "p1",
		Favor: "wtf",
	}
	pod2 := &types.Pod{
		Name:  "p2",
		Favor: "wtf",
	}

	c1 := &types.Container{
		ID:      "c1",
		Podname: "p1",
		Node:    node1,
		Engine:  engine,
		Memory:  5 * types.MByte,
	}

	c2 := &types.Container{
		ID:      "c2",
		Podname: "p2",
		Node:    node2,
		Engine:  engine,
		Memory:  5 * types.MByte,
	}

	containers := []*types.Container{c1, c2}
	store.On("GetContainers", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(containers, nil)
	store.On("GetContainer", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(nil, types.ErrCannotGetEngine).Twice()

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	IDs := []string{"c1", "c2"}
	// GetContainer failed
	ch, err := c.ReallocResource(ctx, IDs, 0.0, 0)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}

	store.On(
		"GetContainer", mock.AnythingOfType("*context.emptyCtx"),
		mock.MatchedBy(func(ID string) bool { return ID == "c1" }),
	).Return(c1, nil)
	store.On(
		"GetContainer", mock.AnythingOfType("*context.emptyCtx"),
		mock.MatchedBy(func(ID string) bool { return ID == "c2" }),
	).Return(c2, nil)
	// GetPod failed
	store.On("GetPod", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Twice()
	ch, err = c.ReallocResource(ctx, IDs, 0.0, 0)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}

	store.On(
		"GetPod", mock.Anything,
		mock.MatchedBy(func(podname string) bool { return podname == "p1" }),
	).Return(pod1, nil).Once()
	store.On(
		"GetPod", mock.Anything,
		mock.MatchedBy(func(podname string) bool { return podname == "p2" }),
	).Return(pod2, nil).Once()
	// favor error
	ch, err = c.ReallocResource(ctx, IDs, 0.0, 0)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}

	// Test memory realloc
	pod1.Favor = scheduler.MEMORY_PRIOR
	pod2.Favor = scheduler.MEMORY_PRIOR
	store.On(
		"GetPod", mock.Anything,
		mock.MatchedBy(func(podname string) bool { return podname == "p1" }),
	).Return(pod1, nil).Times(7) // 根据后面的子测试个数
	store.On(
		"GetPod", mock.Anything,
		mock.MatchedBy(func(podname string) bool { return podname == "p2" }),
	).Return(pod2, nil).Times(7)
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Twice()
	// set memory back failed
	ch, err = c.ReallocResource(ctx, IDs, 0.0, 1)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil).Twice()
	// need memory bigger than node
	ch, err = c.ReallocResource(ctx, IDs, 0.0, types.GByte*2)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}
	// failed because newCPU == 0
	ch, err = c.ReallocResource(ctx, IDs, 0.0, types.MByte*5)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}
	// failed by ContainerUpdate and UpdateNodeResource
	engine.On("ContainerUpdate",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(
		containertypes.ContainerUpdateOKBody{}, types.ErrCannotGetEngine,
	).Twice()
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Twice()
	ch, err = c.ReallocResource(ctx, IDs, 1.0, types.MByte*5)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}
	// failed by UpdateNodeResource
	engine.On("ContainerUpdate", mock.Anything, mock.Anything, mock.Anything).Return(
		containertypes.ContainerUpdateOKBody{}, nil,
	).Times(6)
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Twice()
	ch, err = c.ReallocResource(ctx, IDs, 1.0, -1)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}
	// failed by UpdateContainer
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil).Times(4)
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Twice()
	ch, err = c.ReallocResource(ctx, IDs, 1.0, -1)
	assert.NoError(t, err)
	for msg := range ch {
		assert.False(t, msg.Success)
	}
	// realloc success
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil).Twice()
	ch, err = c.ReallocResource(ctx, IDs, 1.0, -1)
	assert.NoError(t, err)
	for msg := range ch {
		assert.True(t, msg.Success)
	}
}
