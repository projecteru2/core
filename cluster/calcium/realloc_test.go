package calcium

import (
	"context"
	"testing"

	"github.com/projecteru2/core/scheduler"

	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestReallocMem(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	// get lock failed
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.ReallocResource(ctx, []string{"a"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	pod1 := &types.Pod{
		Name:  "p1",
		Favor: "wtf",
	}

	node1 := &types.Node{
		Name:     "node1",
		MemCap:   types.GByte,
		CPU:      types.CPUMap{"0": 50, "1": 70},
		Engine:   engine,
		Endpoint: "http://1.1.1.1:1",
	}

	c1 := &types.Container{
		ID:      "c1",
		Podname: "p1",
		Engine:  engine,
		Memory:  5 * types.MByte,
	}

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetContainer", mock.Anything, mock.Anything).Return(c1, nil)
	// get pod failed
	store.On("GetPod", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	// get node failed
	// pod favor invaild
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrPodNoNodes).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node1, nil)
	// node cap not enough
	pod1.Favor = scheduler.MemoryPrior
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0, 2*types.GByte)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	// new resource invalid
	// update node failed
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	// apply resource failed
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// update container failed
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	origin := node1.MemCap
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	assert.Equal(t, origin-node1.MemCap, int64(1))
	// success
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil)
	origin = node1.MemCap
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, -1)
	assert.NoError(t, err)
	for c := range ch {
		assert.True(t, c.Success)
	}
	assert.Equal(t, node1.MemCap-origin, int64(1))
}

func TestReallocCPU(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	pod1 := &types.Pod{
		Name:  "p1",
		Favor: "wtf",
	}

	node1 := &types.Node{
		Name:     "node1",
		MemCap:   types.GByte,
		CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Engine:   engine,
		Endpoint: "http://1.1.1.1:1",
	}

	c1 := &types.Container{
		ID:      "c1",
		Podname: "p1",
		Engine:  engine,
		Memory:  5 * types.MByte,
		Quota:   0.9,
		CPU:     types.CPUMap{"2": 90},
	}

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetContainer", mock.Anything, mock.Anything).Return(c1, nil)
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node1, nil)
	pod1.Favor = scheduler.CPUPrior
	// wrong cpu
	ch, err := c.ReallocResource(ctx, []string{"c1"}, -1, 2*types.GByte)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	simpleMockScheduler := &schedulermocks.Scheduler{}
	c.scheduler = simpleMockScheduler
	// wrong selectCPUNodes
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientMEM).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 2*types.GByte)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	// wrong total
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, nil).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 2*types.GByte)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	// good to go
	nodeCPUPlans := map[string][]types.CPUMap{
		node1.Name: {
			{
				"3": 100,
			},
			{
				"2": 100,
			},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil)
	// apply resource failed
	// update node failed
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Once()
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	// update container failed
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	origin := node1.MemCap
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0.1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	assert.Equal(t, origin-node1.MemCap, int64(1))
	assert.Equal(t, node1.CPU["3"], 0)
	// success
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil)
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0.1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.True(t, c.Success)
	}
}
