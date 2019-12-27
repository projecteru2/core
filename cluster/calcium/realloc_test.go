package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/docker/go-units"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestRealloc(t *testing.T) {
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
		Name: "p1",
	}

	node1 := &types.Node{
		Name:       "node1",
		MemCap:     int64(units.GiB),
		CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Engine:     engine,
		Endpoint:   "http://1.1.1.1:1",
		NUMA:       types.NUMA{"2": "0"},
		NUMAMemory: types.NUMAMemory{"0": 100000},
	}

	c1 := &types.Container{
		ID:       "c1",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		CPU:      types.CPUMap{"2": 90},
		Nodename: "node1",
	}

	c2 := &types.Container{
		ID:       "c2",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		Nodename: "node1",
	}

	store.On("GetContainers", mock.Anything, []string{"c1"}).Return([]*types.Container{c1}, nil)
	store.On("GetContainers", mock.Anything, []string{"c2"}).Return([]*types.Container{c2}, nil)
	store.On("GetContainers", mock.Anything, []string{"c1", "c2"}).Return([]*types.Container{c1, c2}, nil)
	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.ReallocResource(ctx, []string{"c1"}, -1, 2*int64(units.GiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by GetPod
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, -1, 2*int64(units.GiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	// failed by newCPU < 0
	ch, err = c.ReallocResource(ctx, []string{"c1"}, -1, 2*int64(units.GiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	// failed by GetNode
	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0.1, 2*int64(units.GiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	// failed by memory not enough
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0.1, 2*int64(units.GiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	// failed by no new CPU Plan
	simpleMockScheduler := &schedulermocks.Scheduler{}
	c.scheduler = simpleMockScheduler
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientMEM).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0.1, 2*int64(units.MiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	// failed by wrong total
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, nil).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0.1, 2*int64(units.MiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	// vaild cpu plans
	nodeCPUPlans := map[string][]types.CPUMap{
		node1.Name: {
			{"3": 100},
			{"2": 100},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Twice()
	// failed by apply resource
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Twice()
	// update node failed
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	// update container success
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil).Twice()
	// reset node
	node1 = &types.Node{
		Name:     "node1",
		MemCap:   int64(units.GiB),
		CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Engine:   engine,
		Endpoint: "http://1.1.1.1:1",
	}
	ch, err = c.ReallocResource(ctx, []string{"c1", "c2"}, 0.1, 2*int64(units.MiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	// check node resource as usual
	assert.Equal(t, node1.CPU["2"], 10)
	assert.Equal(t, node1.MemCap, int64(units.GiB))
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	// failed by update container
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Twice()
	ch, err = c.ReallocResource(ctx, []string{"c1", "c2"}, 0.1, 2*int64(units.MiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil).Twice()
	// good to go
	// rest everything
	node2 := &types.Node{
		Name:       "node2",
		MemCap:     int64(units.GiB),
		CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Engine:     engine,
		Endpoint:   "http://1.1.1.1:1",
		NUMA:       types.NUMA{"2": "0"},
		NUMAMemory: types.NUMAMemory{"0": 100000},
	}
	c3 := &types.Container{
		ID:       "c3",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		CPU:      types.CPUMap{"2": 90},
		Nodename: "node2",
	}
	c4 := &types.Container{
		ID:       "c4",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		Nodename: "node2",
	}
	nodeCPUPlans = map[string][]types.CPUMap{
		node2.Name: {
			{"3": 100},
			{"2": 100},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil)
	store.On("GetNode", mock.Anything, "node2").Return(node2, nil)
	store.On("GetContainers", mock.Anything, []string{"c3", "c4"}).Return([]*types.Container{c3, c4}, nil)
	ch, err = c.ReallocResource(ctx, []string{"c3", "c4"}, 0.1, 2*int64(units.MiB), nil)
	assert.NoError(t, err)
	for r := range ch {
		assert.True(t, r.Success)
	}
	assert.Equal(t, node2.CPU["3"], 0)
	assert.Equal(t, node2.CPU["2"], 100)
	assert.Equal(t, node2.MemCap, int64(units.GiB)-4*int64(units.MiB))
}
