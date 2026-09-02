package calcium

import (
	"context"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/log"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestRemapResource(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{"test": {"abc": 123}},
		resourcetypes.Resources{"test": {"abc": 123}},
		[]string{types.ErrMockError.Error()},
		nil,
	)
	rmgr.On("Remap", mock.Anything, mock.Anything, mock.Anything).Return(
		map[string]resourcetypes.Resources{},
		nil,
	)
	engine := &enginemocks.API{}
	node := &types.Node{Engine: engine}

	workload := &types.Workload{
		Resources: resourcetypes.Resources{},
	}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	assert.Nil(t, c.doRemapResource(t.Context(), log.WithField("test", "zc"), node))

	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.RemapResourceAndLog(t.Context(), log.WithField("test", "zc"), node.Name)
}

func TestRemapJournalRetainsUntilEngineSuccess(t *testing.T) {
	for _, tc := range []struct {
		name      string
		engineErr error
		committed bool
	}{
		{name: "success", committed: true},
		{name: "engine failure", engineErr: types.ErrMockError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewTestCluster()
			ctx := t.Context()
			params := resourcetypes.Resources{"cpumem": {"cpu": 2}}

			engine := &enginemocks.API{}
			node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
			workload := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}
			store := c.store.(*storemocks.Store)
			store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil).Once()

			rmgr := c.rmgr.(*resourcemocks.Manager)
			rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
				map[string]resourcetypes.Resources{workload.ID: params}, nil,
			).Once()
			engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, params).Return(tc.engineErr).Once()

			var committed atomic.Bool
			mwal := &walmocks.WAL{}
			mwal.On("Log", eventNodeRemapped, node.Name).Return(wal.Commit(func() error {
				committed.Store(true)
				return nil
			}), nil).Once()
			c.wal = mwal

			err := c.doRemapResource(ctx, log.WithField("test", tc.name), node)
			if tc.engineErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, types.ErrMockError)
			}
			assert.Equal(t, tc.committed, committed.Load())
			store.AssertExpectations(t)
			engine.AssertExpectations(t)
			mwal.AssertExpectations(t)
		})
	}
}

func TestRemapDoesNotFailOnAVanishedWorkload(t *testing.T) {
	c := NewTestCluster()
	params := resourcetypes.Resources{"cpumem": {"cpu": 2}}

	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
	workload := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil).Once()

	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
		map[string]resourcetypes.Resources{workload.ID: params}, nil,
	).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, params).Return(types.ErrWorkloadNotExists).Once()

	assert.NoError(t, c.doRemapResource(t.Context(), log.WithField("test", "gone"), node))
	engine.AssertExpectations(t)
}

func TestRemapReplayRecomputesFromLiveState(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	ctx := t.Context()
	staleParams := resourcetypes.Resources{"cpumem": {"cpu": 1}}
	freshParams := resourcetypes.Resources{"cpumem": {"cpu": 2}}
	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
	workload := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}

	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNode", mock.Anything, node.Name).Return(node, nil)
	store.On("NotFound", mock.Anything).Return(false)
	store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil).Twice()

	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
		map[string]resourcetypes.Resources{workload.ID: staleParams}, nil,
	).Once()
	rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
		map[string]resourcetypes.Resources{workload.ID: freshParams}, nil,
	).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, staleParams).Return(types.ErrMockError).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, freshParams).Return(nil).Once()

	_, err := c.wal.Log(eventNodeRemapped, node.Name)
	require.NoError(t, err)
	c.wal.Recover(ctx)
	c.wal.Recover(ctx)
	c.wal.Recover(ctx)
	store.AssertExpectations(t)
	rmgr.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func TestRemapSkipsWorkloadsWhoseParamsDidNotMove(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	logger := log.WithField("test", "memo")
	params := resourcetypes.Resources{"cpumem": {"cpu": 2, "cpumap": map[string]int{"0": 100}}}
	moved := resourcetypes.Resources{"cpumem": {"cpu": 3, "cpumap": map[string]int{"0": 100}}}

	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
	workload := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil)

	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
		map[string]resourcetypes.Resources{workload.ID: params}, nil,
	).Twice()
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, params).Return(nil).Once()

	assert.NoError(t, c.doRemapResource(ctx, logger, node))
	assert.NoError(t, c.doRemapResource(ctx, logger, node))
	engine.AssertNumberOfCalls(t, "VirtualizationUpdateResource", 1)

	rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
		map[string]resourcetypes.Resources{workload.ID: moved}, nil,
	).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, moved).Return(nil).Once()
	assert.NoError(t, c.doRemapResource(ctx, logger, node))
	engine.AssertNumberOfCalls(t, "VirtualizationUpdateResource", 2)
	rmgr.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func TestRemapForgetsAWorkloadThatLeftTheNode(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	logger := log.WithField("test", "forget")
	params := resourcetypes.Resources{"cpumem": {"cpu": 2}}

	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
	first := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}
	second := &types.Workload{ID: "workload2", Nodename: node.Name, Engine: engine}
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	for _, workload := range []*types.Workload{first, second, first} {
		store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil).Once()
		rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
			map[string]resourcetypes.Resources{workload.ID: params}, nil,
		).Once()
	}
	engine.On("VirtualizationUpdateResource", mock.Anything, first.ID, params).Return(nil).Twice()
	engine.On("VirtualizationUpdateResource", mock.Anything, second.ID, params).Return(nil).Once()

	for range 3 {
		assert.NoError(t, c.doRemapResource(ctx, logger, node))
	}
	memo, _ := c.remapped.Load(node.Name)
	assert.Equal(t, &map[string]uint64{first.ID: hashEngineParams(params)}, memo)
	store.AssertExpectations(t)
	rmgr.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func TestRemapCommitYieldsToAConcurrentInvalidation(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	logger := log.WithField("test", "cas")
	params := resourcetypes.Resources{"cpumem": {"cpu": 2}}

	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
	workload := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
		map[string]resourcetypes.Resources{workload.ID: params}, nil,
	)
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, params).Run(func(mock.Arguments) {
		c.remapped.Delete(node.Name)
	}).Return(nil)

	c.remapped.Store(node.Name, &map[string]uint64{workload.ID: 42})
	assert.NoError(t, c.doRemapResource(ctx, logger, node))
	_, remembered := c.remapped.Load(node.Name)
	assert.False(t, remembered, "a sweep must not resurrect a memo a failed realloc just dropped")
}

func TestHashEngineParamsIsStableAcrossInsertionOrder(t *testing.T) {
	ascending := resourcetypes.Resources{"cpumem": {}}
	for i := range 16 {
		ascending["cpumem"][strconv.Itoa(i)] = i
	}
	descending := resourcetypes.Resources{"cpumem": {}}
	for i := 15; i >= 0; i-- {
		descending["cpumem"][strconv.Itoa(i)] = i
	}
	assert.Equal(t, hashEngineParams(ascending), hashEngineParams(descending))
	assert.NotEqual(t, hashEngineParams(ascending), hashEngineParams(resourcetypes.Resources{"cpumem": {"0": 0}}))
}
