package calcium

import (
	"context"
	"maps"
	"strings"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestHandleWorkloadResourceAllocatedMultipleNodes(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	store.On("GetNode", mock.Anything, mock.Anything).Return(
		func(_ context.Context, name string) *types.Node {
			return &types.Node{NodeMeta: types.NodeMeta{Name: name}}
		}, nil,
	)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, []string{}, nil,
	)

	h := &WorkloadResourceAllocatedHandler{calcium: c}
	nodes := []*types.Node{
		{NodeMeta: types.NodeMeta{Name: "n1"}},
		{NodeMeta: types.NodeMeta{Name: "n2"}},
		{NodeMeta: types.NodeMeta{Name: "n3"}},
		{NodeMeta: types.NodeMeta{Name: "n4"}},
	}
	require.Error(t, h.Handle(t.Context(), nodes))
}

func TestHandleWorkloadResourceAllocatedKeepsEntryUntilEveryNodeIsFixed(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNode", mock.Anything, "n1").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "n1"}}, nil)
	store.On("ListNodeWorkloads", mock.Anything, "n1", mock.Anything).Return(nil, types.ErrMockError).Twice()

	_, err := c.wal.Log(eventWorkloadResourceAllocated, []*types.Node{{NodeMeta: types.NodeMeta{Name: "n1"}}})
	require.NoError(t, err)

	c.wal.Recover(t.Context())
	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
}

func TestHandleCreateWorkloadNoHandle(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		[]string{},
		nil,
	)

	wrkid := "workload-id"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{ID: wrkid, Nodename: "nodename"})
	require.NoError(t, err)

	wrk := &types.Workload{
		ID: "wrkid",
	}

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, nil).Once()
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, nil)

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleCreateWorkloadKeepsEntryOnStoreReadError(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	wrkid := "workload-id"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{ID: wrkid, Nodename: "nodename"})
	require.NoError(t, err)

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, wrkid).Return(nil, types.ErrMockError).Twice()
	store.On("NotFound", types.ErrMockError).Return(false).Twice()
	c.wal.Recover(t.Context())
	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
}

func TestHandleCreateWorkloadHandled(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		[]string{},
		nil,
	)

	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: "nodename"},
		Engine:   &enginemocks.API{},
	}

	wrkid := "workload-id"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{ID: wrkid, Nodename: node.Name})
	require.NoError(t, err)

	wrk := &types.Workload{
		ID:       wrkid,
		Nodename: node.Name,
		Engine:   nil,
	}

	store := c.store.(*storemocks.Store)

	err = errors.Wrapf(types.ErrInvaildCount, "keys: [%s]", wrkid)
	store.On("GetWorkload", mock.Anything, wrkid).Return(nil, err).Once()
	store.On("NotFound", err).Return(true).Once()
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(node, nil)

	eng, ok := node.Engine.(*enginemocks.API)
	require.True(t, ok)
	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(nil).
		Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
	eng.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleCreateWorkloadByNameFromTheStore(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	name := "app_entry_abcdef"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{Name: name, Nodename: "nodename"})
	require.NoError(t, err)

	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, "nodename", mock.Anything).Return(
		[]*types.Workload{{ID: "other", Name: "app_entry_ffffff"}, {ID: "wrkid", Name: name}}, nil,
	).Once()
	store.On("GetWorkloads", mock.Anything, []string{"wrkid"}).Return(nil, nil).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleCreateWorkloadByNameFromTheEngine(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	name := "app_entry_abcdef"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{Name: name, Nodename: "nodename"})
	require.NoError(t, err)

	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, []string{}, nil,
	)

	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "nodename"}, Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, "nodename", mock.Anything).Return(nil, nil).Once()
	store.On("GetNode", mock.Anything, "nodename").Return(node, nil).Once()
	engine.On("VirtualizationInspect", mock.Anything, name).Return(&enginetypes.VirtualizationInfo{ID: "wrkid"}, nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, "wrkid", true, true).Return(nil).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleCreateWorkloadByNameUnknownToBoth(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	name := "app_entry_abcdef"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{Name: name, Nodename: "nodename"})
	require.NoError(t, err)

	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, []string{}, nil,
	)

	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "nodename"}, Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("ListNodeWorkloads", mock.Anything, "nodename", mock.Anything).Return(nil, nil).Once()
	store.On("GetNode", mock.Anything, "nodename").Return(node, nil).Once()
	engine.On("VirtualizationInspect", mock.Anything, name).Return(nil, types.ErrWorkloadNotExists).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleReplaceWorkload(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventWorkloadReplaced, &workloadReplacement{OldID: "old", NewID: "new"})
	require.NoError(t, err)

	engine := &enginemocks.API{}
	oldWorkload := &types.Workload{ID: "old", Name: "a_b_c", Nodename: "nodename", Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, "new").Return(&types.Workload{ID: "new"}, nil).Once()
	store.On("GetWorkload", mock.Anything, "old").Return(oldWorkload, nil).Once()
	store.On("RemoveWorkload", mock.Anything, oldWorkload).Return(nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, "old", true, true).Return(nil).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleReplaceWorkloadKeepsTheOldOneWhenTheNewOneIsGone(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventWorkloadReplaced, &workloadReplacement{OldID: "old", NewID: "new"})
	require.NoError(t, err)

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, "new").Return(nil, types.ErrMockError).Once()
	store.On("NotFound", types.ErrMockError).Return(true).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
}

func TestHandleReallocWorkload(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventWorkloadReallocated, "workloadid")
	require.NoError(t, err)

	engine := &enginemocks.API{}
	engineParams := resourcetypes.Resources{"cpumem": {"cpu": 2}}
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNode", mock.Anything, "n1").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "n1"}}, nil)
	store.On("GetWorkload", mock.Anything, "workloadid").Return(
		&types.Workload{ID: "workloadid", Nodename: "n1", EngineParams: engineParams, Engine: engine}, nil,
	).Twice()
	engine.On("VirtualizationUpdateResource", mock.Anything, "workloadid", engineParams).Return(nil).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleReallocWorkloadOnAnEngineThatCannotReplayIt(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventWorkloadReallocated, "workloadid")
	require.NoError(t, err)

	engine := &enginemocks.API{}
	engineParams := resourcetypes.Resources{"cpumem": {"cpu": 2}}
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNode", mock.Anything, "n1").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "n1"}}, nil)
	store.On("GetWorkload", mock.Anything, "workloadid").Return(
		&types.Workload{ID: "workloadid", Nodename: "n1", EngineParams: engineParams, Engine: engine}, nil,
	).Twice()
	engine.On("VirtualizationUpdateResource", mock.Anything, "workloadid", engineParams).
		Return(types.ErrEngineNotImplemented).Once()

	c.wal.Recover(t.Context())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(t.Context())
}

func TestHandleCreateLambda(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewTestCluster()
		defer c.pool.Release()
		enableTestWAL(t, c)
		rmgr := c.rmgr.(*resourcemocks.Manager)
		rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			resourcetypes.Resources{},
			resourcetypes.Resources{},
			[]string{},
			nil,
		)
		rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			resourcetypes.Resources{},
			resourcetypes.Resources{},
			nil,
		)
		rmgr.On("Remap", mock.Anything, mock.Anything, mock.Anything).Return(
			resourcetypes.Resources{},
			nil,
		)

		_, err := c.wal.Log(eventCreateLambda, "workloadid")
		require.NoError(t, err)

		node := &types.Node{
			NodeMeta: types.NodeMeta{Name: "nodename"},
			Engine:   &enginemocks.API{},
		}
		wrk := &types.Workload{
			ID:       "workloadid",
			Nodename: node.Name,
			Engine:   node.Engine,
		}

		store := c.store.(*storemocks.Store)
		store.On("GetWorkload", mock.Anything, mock.Anything).
			Return(wrk, nil).
			Once()
		store.On("GetNode", mock.Anything, wrk.Nodename).
			Return(node, nil)
		eng := wrk.Engine.(*enginemocks.API)
		eng.On("VirtualizationWait", mock.Anything, wrk.ID, "").Return(&enginetypes.VirtualizationWaitResult{Code: 0}, nil).Once()
		eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
			Return(nil).
			Once()
		store.On("GetWorkloads", mock.Anything, []string{wrk.ID}).
			Return([]*types.Workload{wrk}, nil).
			Twice()
		store.On("RemoveWorkload", mock.Anything, wrk).
			Return(nil).
			Once()
		store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		lock := &lockmocks.DistributedLock{}
		lock.On("Lock", mock.Anything).Return(t.Context(), nil)
		lock.On("Unlock", mock.Anything).Return(nil)
		store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

		c.wal.Recover(t.Context())
		synctest.Wait()
		c.wal.Recover(t.Context())
		synctest.Wait()
		store.AssertExpectations(t)
		eng.AssertExpectations(t)
	})
}

func TestHandleCreateLambdaKeepsEntryUntilRemoved(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewTestCluster()
		defer c.pool.Release()
		enableTestWAL(t, c)

		_, err := c.wal.Log(eventCreateLambda, "workloadid")
		require.NoError(t, err)

		store := c.store.(*storemocks.Store)
		store.On("GetWorkload", mock.Anything, "workloadid").Return(nil, types.ErrMockError).Twice()
		store.On("NotFound", types.ErrMockError).Return(false).Twice()

		c.wal.Recover(t.Context())
		synctest.Wait()
		c.wal.Recover(t.Context())
		synctest.Wait()
		store.AssertExpectations(t)
	})
}

func enableTestWAL(t *testing.T, c *Calcium) {
	mockWALStore(c.store.(*storemocks.Store))
	journal, err := enableWAL(t.Context(), c.config, c, c.store)
	require.NoError(t, err)
	c.wal = journal
}

func mockWALStore(store *storemocks.Store) {
	mutex := &sync.Mutex{}
	data := map[string]string{}

	store.On("Put", mock.Anything, mock.Anything).Return(func(_ context.Context, kvs map[string]string) error {
		mutex.Lock()
		defer mutex.Unlock()
		maps.Copy(data, kvs)
		return nil
	}).Maybe()
	store.On("Delete", mock.Anything, mock.Anything).Return(func(_ context.Context, keys []string) error {
		mutex.Lock()
		defer mutex.Unlock()
		for _, key := range keys {
			delete(data, key)
		}
		return nil
	}).Maybe()
	store.On("GetPrefix", mock.Anything, mock.Anything, mock.Anything).Return(func(_ context.Context, prefix string, _ int64) (map[string]string, error) {
		mutex.Lock()
		defer mutex.Unlock()
		logged := map[string]string{}
		for key, value := range data {
			if strings.HasPrefix(key, prefix) {
				logged[key] = value
			}
		}
		return logged, nil
	}).Maybe()
	store.On("ListPrefix", mock.Anything, mock.Anything).Return(func(_ context.Context, prefix string) ([]string, error) {
		mutex.Lock()
		defer mutex.Unlock()
		keys := []string{}
		for key := range data {
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, key)
			}
		}
		return keys, nil
	}).Maybe()
}
