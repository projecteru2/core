package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	"github.com/projecteru2/core/lock"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource3/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDoLock(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	// create lock failed
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, _, err := c.doLock(ctx, "somename", 1)
	assert.Error(t, err)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// lock failed
	lock.On("Lock", mock.Anything).Return(context.Background(), types.ErrMockError).Once()
	lock.On("Unlock", mock.Anything).Return(nil).Once()
	_, _, err = c.doLock(ctx, "somename", 1)
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	_, _, err = c.doLock(ctx, "somename", 1)
	assert.NoError(t, err)
}

func TestDoUnlockAll(t *testing.T) {
	c := NewTestCluster()
	locks := map[string]lock.DistributedLock{}
	lock := &lockmocks.DistributedLock{}
	locks["somename"] = lock

	// failed
	lock.On("Unlock", mock.Anything).Return(types.ErrMockError)
	c.doUnlockAll(context.Background(), locks)
}

func TestWithWorkloadsLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.Background(), types.ErrMockError).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	err := c.withWorkloadsLocked(ctx, []string{"c1", "c2"}, func(ctx context.Context, workloads map[string]*types.Workload) error { return nil })
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	// failed by getworkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
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
	store := c.store.(*storemocks.Store)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.Background(), types.ErrMockError).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	err := c.withWorkloadLocked(ctx, "c1", func(ctx context.Context, workload *types.Workload) error { return nil })
	assert.Error(t, err)
	// success
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	// failed by getworkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
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

func TestWithNodesPodLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr2.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
			Labels: map[string]string{
				"eru": "1",
			},
			Podname: "test",
		},
		Available: true,
	}
	// failed by list nodes
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, types.ErrMockError).Once()
	err := c.withNodesPodLocked(ctx, &types.NodeFilter{Podname: "test", All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// failed by filter
	var ns map[string]*types.Node
	err = c.withNodesPodLocked(ctx, &types.NodeFilter{Podname: "test", Labels: map[string]string{"eru": "2"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		ns = nodes
		return nil
	})
	assert.NoError(t, err)
	assert.Empty(t, ns)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, nil)
	// failed by getnode
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesPodLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil).Once()
	// failed by lock
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.Background(), types.ErrMockError).Once()
	err = c.withNodesPodLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesPodLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	// success
	err = c.withNodesPodLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		assert.Len(t, nodes, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodePodLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr2.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
			Labels: map[string]string{
				"eru": "1",
			},
			Podname: "test",
		},
		Available: true,
	}
	// failed by lock
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err := c.withNodePodLocked(ctx, "test", func(ctx context.Context, node *types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	// success
	err = c.withNodePodLocked(ctx, "test", func(ctx context.Context, node *types.Node) error {
		assert.Equal(t, node.Name, node1.Name)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodesOperationLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr2.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

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
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, types.ErrMockError).Once()
	err := c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	// failed by filter
	var ns map[string]*types.Node
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Labels: map[string]string{"eru": "2"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		ns = nodes
		return nil
	})
	assert.NoError(t, err)
	assert.Empty(t, ns)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{}, nil)
	// failed by getnode
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil).Once()
	// failed by lock
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed to get lock
	lock.On("Lock", mock.Anything).Return(context.Background(), types.ErrMockError).Once()
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	// success
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		assert.Len(t, nodes, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodeOperationLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr2.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

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
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	// failed by get locked node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err := c.withNodeOperationLocked(ctx, "test", func(ctx context.Context, node *types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	// success
	err = c.withNodeOperationLocked(ctx, "test", func(ctx context.Context, node *types.Node) error {
		assert.Equal(t, node.Name, node1.Name)
		return nil
	})
	assert.NoError(t, err)

	err = c.withNodePodLocked(ctx, "test", func(ctx context.Context, node *types.Node) error {
		return c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
			assert.Equal(t, node.Name, node1.Name)
			return nil
		})
	})
	assert.NoError(t, err)
}
