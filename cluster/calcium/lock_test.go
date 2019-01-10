package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/lock"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestDoLock(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	// create lock failed
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.doLock(ctx, "somename", 1)
	assert.Error(t, err)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// lock failed
	lock.On("Lock", mock.Anything).Return(types.ErrNoETCD).Once()
	_, err = c.doLock(ctx, "somename", 1)
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(nil)
	_, err = c.doLock(ctx, "somename", 1)
	assert.NoError(t, err)
}

func TestDoUnlockAll(t *testing.T) {
	c := NewTestCluster()
	locks := map[string]lock.DistributedLock{}
	lock := &lockmocks.DistributedLock{}
	locks["somename"] = lock

	// failed
	lock.On("Unlock", mock.Anything).Return(types.ErrNoETCD)
	c.doUnlockAll(locks)
}

func TestDoLockAndGetContainers(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(types.ErrNoETCD).Once()
	_, _, _, err := c.doLockAndGetContainers(ctx, []string{"c1", "c2"})
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(nil)
	store.On("GetContainer", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, _, _, err = c.doLockAndGetContainers(ctx, []string{"c1", "c2"})
	assert.Error(t, err)
	engine := &enginemocks.API{}
	container := &types.Container{
		ID:     "c1",
		Engine: engine,
	}
	store.On("GetContainer", mock.Anything, mock.Anything).Return(container, nil)
	// failed by inspect
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, types.ErrNilEngine).Once()
	_, _, _, err = c.doLockAndGetContainers(ctx, []string{"c1", "c2"})
	assert.Error(t, err)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	// success
	containers, containerJSONs, _, err := c.doLockAndGetContainers(ctx, []string{"c1", "c1"})
	assert.NoError(t, err)
	assert.Len(t, containers, 1)
	assert.Len(t, containerJSONs, 1)
}

func TestDoLockAndGetNodes(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	node1 := &types.Node{
		Name: "test",
		Labels: map[string]string{
			"eru": "1",
		},
	}
	// failed by list nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, types.ErrNoETCD).Once()
	_, _, err := c.doLockAndGetNodes(ctx, "test", "", nil)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{node1}, nil)
	// failed by filter
	_, _, err = c.doLockAndGetNodes(ctx, "test", "", map[string]string{"eru": "2"})
	assert.Error(t, err)
	// failed by getnode
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, _, err = c.doLockAndGetNodes(ctx, "test", "test", nil)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node1, nil).Once()
	// failed by lock
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(types.ErrNoETCD).Once()
	_, _, err = c.doLockAndGetNodes(ctx, "test", "test", nil)
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, _, err = c.doLockAndGetNodes(ctx, "test", "test", nil)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node1, nil)
	// success
	nodes, locks, err := c.doLockAndGetNodes(ctx, "test", "test", nil)
	assert.NoError(t, err)
	assert.Len(t, nodes, 1)
	assert.Len(t, locks, 1)
}
