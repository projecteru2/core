package calcium

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
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
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		nil,
	)
	rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{}, nil)

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

func TestRemoveWorkloadReleasesResourcesOnce(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, "a", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationRemove", mock.Anything, "b", mock.Anything, mock.Anything).Return(types.ErrMockError)
	workloads := []*types.Workload{
		{ID: "a", Name: "test", Nodename: "test", Engine: engine, Resources: resourcetypes.Resources{"cpumem": {"cpu": 1}}},
		{ID: "b", Name: "test", Nodename: "test", Engine: engine, Resources: resourcetypes.Resources{"cpumem": {"cpu": 2}}},
	}
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{NodeMeta: types.NodeMeta{Name: "test"}}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	released := mock.MatchedBy(func(r []resourcetypes.Resources) bool { return len(r) == 2 })
	rmgr.On("SetNodeResourceUsage", mock.Anything, "test", mock.Anything, mock.Anything, released, true, plugins.Decr).Return(nil, nil, nil).Once()
	restored := mock.MatchedBy(func(r []resourcetypes.Resources) bool { return len(r) == 1 && r[0]["cpumem"]["cpu"] == 2 })
	rmgr.On("SetNodeResourceUsage", mock.Anything, "test", mock.Anything, mock.Anything, restored, true, plugins.Incr).Return(nil, nil, nil).Once()

	ch, err := c.RemoveWorkload(ctx, []string{"a", "b"}, false)
	assert.NoError(t, err)
	results := map[string]bool{}
	for r := range ch {
		results[r.WorkloadID] = r.Success
	}
	assert.Equal(t, map[string]bool{"a": true, "b": false}, results)
	rmgr.AssertExpectations(t)
}

func TestRemoveWorkloadGivesUnvisitedWorkloadsTheirResourcesBack(t *testing.T) {
	c := NewTestCluster()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	workloads := []*types.Workload{
		{ID: "a", Name: "test", Nodename: "test", Engine: engine, Resources: resourcetypes.Resources{"cpumem": {"cpu": 1}}},
		{ID: "b", Name: "test", Nodename: "test", Engine: engine, Resources: resourcetypes.Resources{"cpumem": {"cpu": 2}}},
	}
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(&types.Node{NodeMeta: types.NodeMeta{Name: "test"}}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	released := mock.MatchedBy(func(r []resourcetypes.Resources) bool { return len(r) == 2 })
	rmgr.On("SetNodeResourceUsage", mock.Anything, "test", mock.Anything, mock.Anything, released, true, plugins.Decr).Run(func(mock.Arguments) { cancel() }).Return(nil, nil, nil).Once()
	restored := mock.MatchedBy(func(r []resourcetypes.Resources) bool { return len(r) == 1 && r[0]["cpumem"]["cpu"] == 2 })
	rmgr.On("SetNodeResourceUsage", mock.Anything, "test", mock.Anything, mock.Anything, restored, true, plugins.Incr).Return(nil, nil, nil).Once()

	ch, err := c.RemoveWorkload(ctx, []string{"a", "b"}, false)
	assert.NoError(t, err)
	time.Sleep(500 * time.Millisecond)
	for range ch { //nolint:revive
	}
	rmgr.AssertExpectations(t)
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
	lock.On("Lock", mock.Anything).Return(ctx, nil)
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
