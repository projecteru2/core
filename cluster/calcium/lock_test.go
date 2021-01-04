package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/engine/mocks"
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
	_, _, err := c.doLock(ctx, "somename", 1)
	assert.Error(t, err)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// lock failed
	lock.On("Lock", mock.Anything).Return(context.TODO(), types.ErrNoETCD).Once()
	_, _, err = c.doLock(ctx, "somename", 1)
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	_, _, err = c.doLock(ctx, "somename", 1)
	assert.NoError(t, err)
}

func TestDoUnlockAll(t *testing.T) {
	c := NewTestCluster()
	locks := map[string]lock.DistributedLock{}
	lock := &lockmocks.DistributedLock{}
	locks["somename"] = lock

	// failed
	lock.On("Unlock", mock.Anything).Return(types.ErrNoETCD)
	c.doUnlockAll(context.Background(), locks)
}

func TestWithWorkloadsLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.TODO(), types.ErrNoETCD).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	err := c.withWorkloadsLocked(ctx, []string{"c1", "c2"}, func(ctx context.Context, workloads map[string]*types.Workload) error { return nil })
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	// failed by getworkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err = c.withWorkloadsLocked(ctx, []string{"c1", "c2"}, func(ctx context.Context, workloads map[string]*types.Workload) error { return nil })
	assert.Error(t, err)
	engine := &enginemocks.API{}
	workload := &types.Workload{
		ID:     "c1",
		Engine: engine,
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	// success
	err = c.withWorkloadsLocked(ctx, []string{"c1", "c1"}, func(ctx context.Context, workloads map[string]*types.Workload) error {
		assert.Len(t, workloads, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithWorkloadLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.TODO(), types.ErrNoETCD).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	err := c.withWorkloadLocked(ctx, "c1", func(ctx context.Context, workload *types.Workload) error { return nil })
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	// failed by getworkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err = c.withWorkloadLocked(ctx, "c1", func(ctx context.Context, workload *types.Workload) error { return nil })
	assert.Error(t, err)
	engine := &enginemocks.API{}
	workload := &types.Workload{
		ID:     "c1",
		Engine: engine,
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	// success
	err = c.withWorkloadLocked(ctx, "c1", func(ctx context.Context, workload *types.Workload) error {
		assert.Equal(t, workload.ID, "c1")
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodesLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
			Labels: map[string]string{
				"eru": "1",
			},
		},
		Available: true,
	}
	// failed by list nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, types.ErrNoETCD).Once()
	err := c.withNodesLocked(ctx, "test", nil, nil, false, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// failed by filter
	var ns map[string]*types.Node
	err = c.withNodesLocked(ctx, "test", nil, map[string]string{"eru": "2"}, false, func(ctx context.Context, nodes map[string]*types.Node) error {
		ns = nodes
		return nil
	})
	assert.NoError(t, err)
	assert.Empty(t, ns)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil)
	// failed by getnode
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err = c.withNodesLocked(ctx, "test", []string{"test"}, nil, false, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil).Once()
	// failed by lock
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.TODO(), types.ErrNoETCD).Once()
	err = c.withNodesLocked(ctx, "test", []string{"test"}, nil, false, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err = c.withNodesLocked(ctx, "test", []string{"test"}, nil, false, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	// success
	err = c.withNodesLocked(ctx, "test", []string{"test"}, nil, false, func(ctx context.Context, nodes map[string]*types.Node) error {
		assert.Len(t, nodes, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodeLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
			Labels: map[string]string{
				"eru": "1",
			},
		},
		Available: true,
	}
	// failed by lock
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err := c.withNodeLocked(ctx, "test", func(ctx context.Context, node *types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	// success
	err = c.withNodeLocked(ctx, "test", func(ctx context.Context, node *types.Node) error {
		assert.Equal(t, node.Name, node1.Name)
		return nil
	})
	assert.NoError(t, err)
}
