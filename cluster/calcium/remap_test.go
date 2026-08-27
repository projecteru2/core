package calcium

import (
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
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.RemapResourceAndLog(t.Context(), log.WithField("test", "zc"), node)
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
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
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
