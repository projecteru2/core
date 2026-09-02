package calcium

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	"github.com/projecteru2/core/lock"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestDoLock(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, _, err := c.doLock(ctx, "somename", 1)
	assert.Error(t, err)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(t.Context(), types.ErrMockError).Once()
	lock.On("Unlock", mock.Anything).Return(nil).Once()
	_, _, err = c.doLock(ctx, "somename", 1)
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	_, _, err = c.doLock(ctx, "somename", 1)
	assert.NoError(t, err)
}

func TestDoLockUnlocksWithDetachedContext(t *testing.T) {
	c := NewTestCluster()
	ctx, cancel := context.WithCancel(t.Context())
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(t.Context(), types.ErrMockError).Once()

	var unlockCtxErr error
	lock.On("Unlock", mock.Anything).Return(nil).Once().Run(func(args mock.Arguments) {
		unlockCtxErr = args.Get(0).(context.Context).Err()
	})

	cancel()
	_, _, err := c.doLock(ctx, "somename", time.Minute)
	assert.Error(t, err)
	assert.NoError(t, unlockCtxErr)
	lock.AssertExpectations(t)
}

func TestDoUnlockAll(t *testing.T) {
	c := NewTestCluster()
	locks := map[string]lock.DistributedLock{}
	lock := &lockmocks.DistributedLock{}
	locks["somename"] = lock

	lock.On("Unlock", mock.Anything).Return(types.ErrMockError)
	c.doUnlockAll(t.Context(), locks, []string{"somename"})
}

func TestWithWorkloadsLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(t.Context(), types.ErrMockError).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	err := c.withWorkloadsLocked(ctx, false, []string{"c1", "c2"}, func(ctx context.Context, workloads map[string]*types.Workload) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withWorkloadsLocked(ctx, false, []string{"c1", "c2"}, func(ctx context.Context, workloads map[string]*types.Workload) error { return nil })
	assert.Error(t, err)
	engine := &enginemocks.API{}
	workload := &types.Workload{
		ID:     "c1",
		Engine: engine,
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	err = c.withWorkloadsLocked(ctx, false, []string{"c1", "c1"}, func(ctx context.Context, workloads map[string]*types.Workload) error {
		assert.Len(t, workloads, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithWorkloadLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(t.Context(), types.ErrMockError).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	err := c.withWorkloadLocked(ctx, "c1", false, func(ctx context.Context, workload *types.Workload) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withWorkloadLocked(ctx, "c1", false, func(ctx context.Context, workload *types.Workload) error { return nil })
	assert.Error(t, err)
	engine := &enginemocks.API{}
	workload := &types.Workload{
		ID:     "c1",
		Engine: engine,
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	err = c.withWorkloadLocked(ctx, "c1", false, func(ctx context.Context, workload *types.Workload) error {
		assert.Equal(t, workload.ID, "c1")
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodesPlanLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, types.ErrMockError).Once()
	err := c.withNodesPlanLocked(ctx, &types.NodeFilter{Podname: "test", All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	var ns map[string]*types.Node
	err = c.withNodesPlanLocked(ctx, &types.NodeFilter{Podname: "test", Labels: map[string]string{"eru": "2"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		ns = nodes
		return nil
	})
	assert.NoError(t, err)
	assert.Empty(t, ns)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesPlanLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil).Once()
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(t.Context(), types.ErrMockError).Once()
	err = c.withNodesPlanLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesPlanLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	err = c.withNodesPlanLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		assert.Len(t, nodes, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodesPlanLockedTakesPodAndNodeLocksInKeyOrder(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	nodes := []*types.Node{
		{NodeMeta: types.NodeMeta{Name: "a1", Podname: "podb"}, Available: true},
		{NodeMeta: types.NodeMeta{Name: "b1", Podname: "poda"}, Available: true},
		{NodeMeta: types.NodeMeta{Name: "c1", Podname: "podb"}, Available: true},
	}
	store.On("GetNodes", mock.Anything, mock.Anything).Return(nodes, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	keys := []string{}
	store.On("CreateLock", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		keys = append(keys, args.String(0))
	}).Return(lock, nil)

	err := c.withNodesPlanLocked(t.Context(), &types.NodeFilter{Includes: []string{"a1", "b1", "c1"}}, func(_ context.Context, locked map[string]*types.Node) error {
		assert.Len(t, locked, 3)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"cnode_op_poda_b1", "cnode_op_podb_a1", "cnode_op_podb_c1", "plock_poda", "plock_podb"}, keys)
}

func TestWithNodesPlanLockedTakesOnlyTheNodeLockForOneCandidate(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	store.On("GetNode", mock.Anything, "a1").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "a1", Podname: "poda"}, Available: true}, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	keys := []string{}
	store.On("CreateLock", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		keys = append(keys, args.String(0))
	}).Return(lock, nil)

	err := c.withNodesPlanLocked(t.Context(), &types.NodeFilter{Includes: []string{"a1"}}, func(_ context.Context, locked map[string]*types.Node) error {
		assert.Len(t, locked, 1)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"cnode_op_poda_a1"}, keys)
}

func TestWithNodesOperationLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, types.ErrMockError).Once()
	err := c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil).Once()
	var ns map[string]*types.Node
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Labels: map[string]string{"eru": "2"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		ns = nodes
		return nil
	})
	assert.NoError(t, err)
	assert.Empty(t, ns)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil).Once()
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(t.Context(), types.ErrMockError).Once()
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	err = c.withNodesOperationLocked(ctx, &types.NodeFilter{Podname: "test", Includes: []string{"test"}, All: false}, func(ctx context.Context, nodes map[string]*types.Node) error {
		assert.Len(t, nodes, 1)
		return nil
	})
	assert.NoError(t, err)
}

func TestWithNodeOperationLocked(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
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
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err := c.withNodeOperationLocked(ctx, "test", func(ctx context.Context, node *types.Node) error { return nil })
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	err = c.withNodeOperationLocked(ctx, "test", func(ctx context.Context, node *types.Node) error {
		assert.Equal(t, node.Name, node1.Name)
		return nil
	})
	assert.NoError(t, err)

	err = c.withNodeOperationLocked(ctx, "test", func(ctx context.Context, node *types.Node) error {
		return c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
			assert.Equal(t, node.Name, node1.Name)
			return nil
		})
	})
	assert.NoError(t, err)
}
