package calcium

import (
	"context"
	"testing"

	"github.com/projecteru2/core/scheduler"

	enginetypes "github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/3rdmocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestReallocMem(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()

	c.store = store
	// get lock failed
	ch, err := c.ReallocResource(ctx, []string{"a"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.APIClient{}
	engine.On("ContainerInspect", mock.Anything, mock.Anything).Return(enginetypes.ContainerJSON{}, nil)

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
		HostIP:  node1.GetIP(),
	}

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetContainer", mock.Anything, mock.Anything).Return(c1, nil)
	store.On("GetPod", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	// get pod failed
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)

	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrPodNoNodes).Once()
	// get node failed
	// pod favor invaild
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 0, 0)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	// node cap not enough
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node1, nil)
	pod1.Favor = scheduler.MEMORY_PRIOR
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
	engine.On("ContainerUpdate", mock.Anything, mock.Anything, mock.Anything).Return(containertypes.ContainerUpdateOKBody{}, types.ErrBadContainerID).Once()
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	engine.On("ContainerUpdate", mock.Anything, mock.Anything, mock.Anything).Return(containertypes.ContainerUpdateOKBody{}, nil)
	// update container failed
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	origin := node1.MemCap
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, 1)
	assert.NoError(t, err)
	for c := range ch {
		assert.False(t, c.Success)
	}
	assert.Equal(t, origin-node1.MemCap, int64(1))
	// sucess
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil)
	origin = node1.MemCap
	ch, err = c.ReallocResource(ctx, []string{"c1"}, 1, -1)
	assert.NoError(t, err)
	for c := range ch {
		assert.True(t, c.Success)
	}
	assert.Equal(t, node1.MemCap-origin, int64(1))
}
