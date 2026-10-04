package calcium

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/engine/factory"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/store"
	storemocks "github.com/projecteru2/core/store/mocks"
	redisstore "github.com/projecteru2/core/store/redis"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestRemoveWorkload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewTestCluster()
		defer c.pool.Release()
		ctx := t.Context()
		lock := heldLock(t)
		store := c.store.(*storemocks.Store)
		rmgr := c.rmgr.(*resourcemocks.Manager)
		rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
		rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			resourcetypes.Resources{},
			resourcetypes.Resources{},
			nil,
		)

		store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
		ch, err := c.RemoveWorkload(ctx, []string{"xx"}, false)
		assert.True(t, errors.Is(err, types.ErrMockError))
		store.AssertExpectations(t)

		workload := &types.Workload{
			ID:       "xx",
			Name:     "test",
			Nodename: "test",
		}
		store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
		store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
		ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false)
		assert.NoError(t, err)
		for r := range ch {
			assert.False(t, r.Success)
		}
		synctest.Wait()
		store.AssertExpectations(t)

		store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
		store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
		node := &types.Node{
			NodeMeta: types.NodeMeta{
				Name: "test",
			},
		}
		store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
		store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(types.ErrMockError).Twice()
		store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
		ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false)
		assert.NoError(t, err)
		for r := range ch {
			assert.False(t, r.Success)
		}
		assert.Error(t, c.doRemoveWorkloadSync(ctx, []string{"xx"}))
		synctest.Wait()
		store.AssertExpectations(t)

		engine := &enginemocks.API{}
		workload.Engine = engine
		engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
		store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
		store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
		ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false)
		assert.NoError(t, err)
		for r := range ch {
			assert.True(t, r.Success)
		}
		store.AssertExpectations(t)
	})
}

func TestRemoveWorkloadReportsEveryWorkloadAfterTheLockIsLost(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	lostCtx, lose := context.WithCancel(ctx)
	lose()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(lostCtx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workloads := []*types.Workload{
		{ID: "a", Name: "test", Nodename: "test", Engine: engine},
		{ID: "b", Name: "test", Nodename: "test", Engine: engine},
	}
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(func(_ context.Context, ID string) (*types.Workload, error) {
		for _, w := range workloads {
			if w.ID == ID {
				return w, nil
			}
		}
		return nil, types.ErrWorkloadNotExists
	})
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{NodeMeta: types.NodeMeta{Name: "test"}}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.Resources{}, resourcetypes.Resources{}, nil)

	ch, err := c.RemoveWorkload(ctx, []string{"a", "b"}, false)
	assert.NoError(t, err)
	reported := []string{}
	for r := range ch {
		reported = append(reported, r.WorkloadID)
	}
	assert.ElementsMatch(t, []string{"a", "b"}, reported)
}

func TestRemoveWorkloadJournalsRepairEntries(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	logged := []string{}
	committed := 0
	mwal := &walmocks.WAL{}
	mwal.On("Log", mock.Anything, mock.Anything).Return(func(eventyp string, _ any) (wal.Commit, error) {
		logged = append(logged, eventyp)
		return func() error { committed++; return nil }, nil
	})
	c.wal = mwal

	lock := heldLock(t)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workload := &types.Workload{ID: "xx", Name: "test", Nodename: "test", Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{NodeMeta: types.NodeMeta{Name: "test"}}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, nil,
	)

	ch, err := c.RemoveWorkload(ctx, []string{"xx"}, true)
	assert.NoError(t, err)
	for r := range ch {
		assert.True(t, r.Success)
	}
	assert.Equal(t, []string{eventWorkloadResourceAllocated, eventWorkloadCreated}, logged)
	assert.Equal(t, 2, committed)
}

func TestRemoveWorkloadKeepsTheNodeEntryWhenTheReleaseFails(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	committed := []string{}
	mwal := &walmocks.WAL{}
	mwal.On("Log", mock.Anything, mock.Anything).Return(func(eventyp string, _ any) (wal.Commit, error) {
		return func() error { committed = append(committed, eventyp); return nil }, nil
	})
	c.wal = mwal

	lock := heldLock(t)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workload := &types.Workload{ID: "xx", Name: "test", Nodename: "test", Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{NodeMeta: types.NodeMeta{Name: "test"}}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil, nil, types.ErrMockError,
	)

	ch, err := c.RemoveWorkload(ctx, []string{"xx"}, true)
	assert.NoError(t, err)
	for r := range ch {
		assert.True(t, r.Success)
	}
	engine.AssertExpectations(t)
	assert.Equal(t, []string{eventWorkloadCreated}, committed)
}

func TestRemoveWorkloadKeepsTheNodeEntryWhenTheRemovalFails(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	committed := []string{}
	mwal := &walmocks.WAL{}
	mwal.On("Log", mock.Anything, mock.Anything).Return(func(eventyp string, _ any) (wal.Commit, error) {
		return func() error { committed = append(committed, eventyp); return nil }, nil
	})
	c.wal = mwal

	lock := heldLock(t)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(types.ErrMockError)
	workload := &types.Workload{ID: "xx", Name: "test", Nodename: "test", Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{NodeMeta: types.NodeMeta{Name: "test"}}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, nil,
	)

	ch, err := c.RemoveWorkload(ctx, []string{"xx"}, true)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	engine.AssertExpectations(t)
	assert.Equal(t, []string{eventWorkloadCreated}, committed)
}

func TestRemoveWorkloadLocksTheWorkloadThenItsNode(t *testing.T) {
	c := NewTestCluster()
	defer c.pool.Release()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workload := &types.Workload{ID: "w1", Name: "app_entry_x", Nodename: "n1", Engine: engine}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1", Podname: "p1"}}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetNode", mock.Anything, "n1").Return(node, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.Resources{}, resourcetypes.Resources{}, nil)
	rmgr.On("Remap", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	lock := heldLock(t)
	var mu sync.Mutex
	keys := []string{}
	store.On("CreateLock", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		mu.Lock()
		defer mu.Unlock()
		keys = append(keys, args.String(0))
	}).Return(lock, nil)

	ch, err := c.RemoveWorkload(ctx, []string{"w1"}, true)
	assert.NoError(t, err)
	for m := range ch {
		assert.True(t, m.Success)
	}
	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, []string{"clock_w1", "cnode_op_p1_n1"}, keys[:2], "the workload lock first, then the node lock around the release, never the pod lock")
	assert.NotContains(t, keys, "plock_p1")
}

func TestRemoveWorkloadCommitsOnlyAfterTheOutcomeSettles(t *testing.T) {
	tests := []struct {
		name       string
		deleted    bool
		readErr    error
		restoreErr error
		engineErr  error
		wantCommit bool
	}{
		{name: "delete never applied", wantCommit: true},
		{name: "delete applied but reply lost", deleted: true, wantCommit: true},
		{name: "delete outcome still unknown", readErr: types.ErrMockError},
		{name: "delete applied but restoration fails", deleted: true, restoreErr: types.ErrMockError},
		{name: "engine and restoration fail", engineErr: types.ErrMockError, restoreErr: types.ErrMockError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewTestCluster()
			t.Cleanup(c.Finalizer)
			engine := &enginemocks.API{}
			workload := &types.Workload{ID: "w1", Name: "app_web_123456", Nodename: "n1", Engine: engine}
			committed := false
			mwal := &walmocks.WAL{}
			mwal.On("Log", eventWorkloadCreated, mock.Anything).Return(wal.Commit(func() error { committed = true; return nil }), nil).Once()
			mwal.On("Close").Return(nil).Maybe()
			c.wal = mwal
			store := c.store.(*storemocks.Store)
			if tt.engineErr != nil {
				store.On("RemoveWorkload", mock.Anything, workload).Return(nil).Once()
				engine.On("VirtualizationRemove", mock.Anything, workload.ID, true, true).Return(tt.engineErr).Once()
				store.On("AddWorkload", mock.Anything, workload, mock.Anything).Return(tt.restoreErr).Once()
			} else {
				store.On("RemoveWorkload", mock.Anything, workload).Return(context.DeadlineExceeded).Once()
				switch {
				case tt.readErr != nil:
					store.On("GetWorkload", mock.Anything, workload.ID).Return(nil, tt.readErr).Once()
					store.On("NotFound", tt.readErr).Return(false).Once()
				case tt.deleted:
					store.On("GetWorkload", mock.Anything, workload.ID).Return(nil, types.ErrKeyNotFound).Once()
					store.On("NotFound", types.ErrKeyNotFound).Return(true).Once()
					store.On("AddWorkload", mock.Anything, workload, mock.Anything).Return(tt.restoreErr).Once()
				default:
					store.On("GetWorkload", mock.Anything, workload.ID).Return(workload, nil).Once()
				}
			}
			require.Error(t, c.doRemoveOneWorkload(t.Context(), workload, true))
			assert.Equal(t, tt.wantCommit, committed)
			store.AssertExpectations(t)
			engine.AssertExpectations(t)
			mwal.AssertExpectations(t)
		})
	}
}

func TestRemoveWorkloadRestoresMetadataAfterDeleteReplyLoss(t *testing.T) {
	c := NewTestCluster()
	t.Cleanup(c.Finalizer)
	srv := miniredis.RunT(t)
	c.config.Redis.Addr = srv.Addr()
	c.config.ConnectionTimeout = time.Second
	factory.InitEngineCache(t.Context(), c.config, nil)
	backend, err := redisstore.New(c.config)
	require.NoError(t, err)
	ctx := t.Context()
	_, err = backend.AddPod(ctx, "p1", "")
	require.NoError(t, err)
	node, err := backend.AddNode(ctx, &types.AddNodeOptions{Podname: "p1", Nodename: "n1", Endpoint: "mock://n1"})
	require.NoError(t, err)
	engine := &enginemocks.API{}
	workload := &types.Workload{ID: "w1", Name: "app_web_123456", Podname: "p1", Nodename: node.Name, Engine: engine}
	require.NoError(t, backend.AddWorkload(ctx, workload, nil))
	c.store = &deleteReplyLostStore{Store: backend}
	c.wal, err = enableWAL(ctx, c.config, c, backend)
	require.NoError(t, err)

	require.ErrorIs(t, c.doRemoveOneWorkload(ctx, workload, true), context.DeadlineExceeded)
	restored, err := backend.GetWorkload(ctx, workload.ID)
	require.NoError(t, err)
	require.Equal(t, workload.ID, restored.ID)
	entries, err := backend.GetPrefix(ctx, "/wal/", 0)
	require.NoError(t, err)
	require.Empty(t, entries)
	engine.AssertNotCalled(t, "VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything)

	engine.On("VirtualizationRemove", mock.Anything, workload.ID, true, true).Return(nil).Once()
	require.NoError(t, c.doRemoveOneWorkload(ctx, workload, true))
	_, err = backend.GetWorkload(ctx, workload.ID)
	require.ErrorIs(t, err, types.ErrWorkloadNotExists)
	engine.AssertExpectations(t)
}

func TestReplaceRecoveryRemovesTheEngineAfterUncertainStoreDeletion(t *testing.T) {
	for _, newPresent := range []bool{false, true} {
		t.Run(fmt.Sprintf("new_present_%t", newPresent), func(t *testing.T) {
			c := NewTestCluster()
			t.Cleanup(c.Finalizer)
			srv := miniredis.RunT(t)
			c.config.Redis.Addr = srv.Addr()
			c.config.ConnectionTimeout = time.Second
			factory.InitEngineCache(t.Context(), c.config, nil)
			backend, err := redisstore.New(c.config)
			require.NoError(t, err)
			ctx := t.Context()
			_, err = backend.AddPod(ctx, "p1", "")
			require.NoError(t, err)
			node, err := backend.AddNode(ctx, &types.AddNodeOptions{Podname: "p1", Nodename: "n1", Endpoint: "mock://n1"})
			require.NoError(t, err)
			engine := &enginemocks.API{}
			node.Engine = engine
			workload := &types.Workload{ID: "old", Name: "app_web_old", Podname: "p1", Nodename: node.Name, Engine: engine}
			require.NoError(t, backend.AddWorkload(ctx, workload, nil))
			if newPresent {
				require.NoError(t, backend.AddWorkload(ctx, &types.Workload{ID: "new", Name: "app_web_new", Podname: "p1", Nodename: node.Name}, nil))
			}
			faulty := &deleteReplyLostStore{Store: backend, readFailure: true, node: node}
			c.store = faulty
			c.rmgr.(*resourcemocks.Manager).On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.Resources{}, resourcetypes.Resources{}, []string{}, nil)
			c.wal, err = enableWAL(ctx, c.config, c, backend)
			require.NoError(t, err)
			_, err = c.wal.Log(eventWorkloadReplaced, &workloadReplacement{OldID: workload.ID, NewID: "new", Nodename: node.Name})
			require.NoError(t, err)

			settled, err := c.doRemoveWorkload(ctx, workload, true)
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.False(t, settled)
			_, err = backend.GetWorkload(ctx, workload.ID)
			require.ErrorIs(t, err, types.ErrWorkloadNotExists)
			engine.AssertNotCalled(t, "VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			entries, err := backend.GetPrefix(ctx, "/wal/", 0)
			require.NoError(t, err)
			require.Len(t, entries, 1)

			faulty.readFailure = false
			engine.On("VirtualizationRemove", mock.Anything, workload.ID, true, true).Return(nil).Once()
			c.wal.Recover(ctx)
			entries, err = backend.GetPrefix(ctx, "/wal/", 0)
			require.NoError(t, err)
			require.Empty(t, entries)
			c.wal.Recover(ctx)
			engine.AssertExpectations(t)
		})
	}
}

type deleteReplyLostStore struct {
	store.Store
	failed      bool
	readFailure bool
	node        *types.Node
}

func (s *deleteReplyLostStore) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	if err := s.Store.RemoveWorkload(ctx, workload); err != nil {
		return err
	}
	if !s.failed {
		s.failed = true
		return context.DeadlineExceeded
	}
	return nil
}

func (s *deleteReplyLostStore) GetWorkload(ctx context.Context, ID string) (*types.Workload, error) {
	if s.failed && s.readFailure {
		return nil, context.DeadlineExceeded
	}
	return s.Store.GetWorkload(ctx, ID)
}

func (s *deleteReplyLostStore) GetNode(ctx context.Context, name string) (*types.Node, error) {
	if s.node != nil && s.node.Name == name {
		return s.node, nil
	}
	return s.Store.GetNode(ctx, name)
}
