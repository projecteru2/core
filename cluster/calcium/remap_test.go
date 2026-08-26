package calcium

import (
	"context"
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
	assert.Nil(t, c.doRemapResource(context.Background(), log.WithField("test", "zc"), node))

	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.RemapResourceAndLog(context.Background(), log.WithField("test", "zc"), node)
}

func TestRemapResourcePersistsDesiredParamsBeforeEngineUpdate(t *testing.T) {
	for _, tc := range []struct {
		name      string
		storeErr  error
		engineErr error
		committed bool
	}{
		{name: "success", committed: true},
		{name: "store failure", storeErr: types.ErrMockError},
		{name: "engine failure", engineErr: types.ErrMockError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewTestCluster()
			ctx := context.Background()
			params := resourcetypes.Resources{"cpumem": {"cpu": 2}}

			engine := &enginemocks.API{}
			node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
			workload := &types.Workload{ID: "workload1", Nodename: node.Name, Engine: engine}
			store := c.store.(*storemocks.Store)
			store.On("ListNodeWorkloads", mock.Anything, node.Name, mock.Anything).Return([]*types.Workload{workload}, nil).Once()
			store.On("GetWorkload", mock.Anything, workload.ID).Return(workload, nil).Once()
			var updated *types.Workload
			store.On("UpdateWorkload", mock.Anything, mock.Anything).
				Run(func(args mock.Arguments) { updated = args.Get(1).(*types.Workload) }).
				Return(tc.storeErr).
				Once()

			rmgr := c.rmgr.(*resourcemocks.Manager)
			rmgr.On("Remap", mock.Anything, node.Name, mock.Anything).Return(
				map[string]resourcetypes.Resources{workload.ID: params}, nil,
			).Once()
			if tc.storeErr == nil {
				engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, params).Return(tc.engineErr).Once()
			}

			var committed atomic.Bool
			mwal := &walmocks.WAL{}
			mwal.On("Log", eventWorkloadRemapped, &workloadRemap{ID: workload.ID, EngineParams: params}).Return(wal.Commit(func() error {
				committed.Store(true)
				return nil
			}), nil).Once()
			c.wal = mwal

			err := c.doRemapResource(ctx, log.WithField("test", tc.name), node)
			if tc.storeErr == nil && tc.engineErr == nil {
				assert.NoError(t, err)
			} else {
				assert.ErrorIs(t, err, types.ErrMockError)
			}
			require.NotNil(t, updated)
			assert.Equal(t, params, updated.EngineParams)
			assert.Equal(t, tc.committed, committed.Load())
			mwal.AssertExpectations(t)
		})
	}
}

func TestRemapWorkloadJournalRetriesEngineFailure(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	ctx := context.Background()
	params := resourcetypes.Resources{"cpumem": {"cpu": 2}}
	engine := &enginemocks.API{}
	workload := &types.Workload{ID: "workload1", Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, workload.ID).Return(workload, nil).Twice()
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil).Twice()
	remappedParams := mock.MatchedBy(func(params resourcetypes.Resources) bool {
		return params["cpumem"].Int("cpu") == 2
	})
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, remappedParams).Return(types.ErrMockError).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, workload.ID, remappedParams).Return(nil).Once()

	_, err := c.wal.Log(eventWorkloadRemapped, &workloadRemap{ID: workload.ID, EngineParams: params})
	require.NoError(t, err)
	c.wal.Recover(ctx)
	c.wal.Recover(ctx)
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
}
