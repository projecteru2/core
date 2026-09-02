package calcium

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestRemoveWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
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
	time.Sleep(time.Second)
	store.AssertExpectations(t)

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
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
	time.Sleep(time.Second)
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

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workload := &types.Workload{ID: "xx", Name: "test", Nodename: "test", Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
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

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workload := &types.Workload{ID: "xx", Name: "test", Nodename: "test", Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
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
	store.On("GetNode", mock.Anything, "n1").Return(node, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.Resources{}, resourcetypes.Resources{}, nil)
	rmgr.On("Remap", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
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
