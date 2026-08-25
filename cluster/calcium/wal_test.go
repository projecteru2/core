package calcium

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestHandleWorkloadResourceAllocatedMultipleNodes(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
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

	h := newWorkloadResourceAllocatedHandler(c)
	nodes := []*types.Node{
		{NodeMeta: types.NodeMeta{Name: "n1"}},
		{NodeMeta: types.NodeMeta{Name: "n2"}},
		{NodeMeta: types.NodeMeta{Name: "n3"}},
		{NodeMeta: types.NodeMeta{Name: "n4"}},
	}
	require.NoError(t, h.Handle(context.Background(), nodes))
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

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)

	c.wal.Recover(context.Background())
}

func TestHandleCreateWorkloadError(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		[]string{},
		nil,
	)

	engine := &enginemocks.API{}
	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: "nodename"},
		Engine:   engine,
	}
	wrkid := "workload-id"
	_, err := c.wal.Log(eventWorkloadCreated, &types.Workload{ID: wrkid, Nodename: node.Name})
	require.NoError(t, err)

	wrk := &types.Workload{
		ID:       wrkid,
		Nodename: node.Name,
	}

	store := c.store.(*storemocks.Store)

	err = errors.Wrapf(types.ErrInvaildCount, "keys: [%s]", wrkid)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(wrk, err).Once()
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, err).Once()
	store.On("NotFound", err).Return(false).Once()
	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(wrk, err).Once()
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(node, nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(err).Once()
	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, fmt.Errorf("err")).Once()
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(types.ErrWorkloadNotExists).Once()
	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	_, err = c.wal.Log(eventWorkloadCreated, &types.Workload{ID: wrkid, Nodename: node.Name})
	require.NoError(t, err)
	gone := fmt.Errorf("node gone")
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, gone).Once()
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(nil, gone).Once()
	store.On("NotFound", gone).Return(true).Once()
	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
	c.wal.Recover(context.Background())
	store.AssertExpectations(t)

	c.wal.Recover(context.Background())
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
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(node, nil)

	eng, ok := node.Engine.(*enginemocks.API)
	require.True(t, ok)
	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(nil).
		Once()

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	eng.AssertExpectations(t)

	c.wal.Recover(context.Background())
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

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)

	c.wal.Recover(context.Background())
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

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(context.Background())
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

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(context.Background())
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

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(context.Background())
}

func TestHandleReplaceWorkloadKeepsTheOldOneWhenTheNewOneIsGone(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventWorkloadReplaced, &workloadReplacement{OldID: "old", NewID: "new"})
	require.NoError(t, err)

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, "new").Return(nil, types.ErrMockError).Once()
	store.On("NotFound", types.ErrMockError).Return(true).Once()

	c.wal.Recover(context.Background())
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
	store.On("GetWorkload", mock.Anything, "workloadid").Return(
		&types.Workload{ID: "workloadid", EngineParams: engineParams, Engine: engine}, nil,
	).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, "workloadid", engineParams).Return(nil).Once()

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(context.Background())
}

func TestHandleReallocWorkloadOnAnEngineThatCannotReplayIt(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventWorkloadReallocated, "workloadid")
	require.NoError(t, err)

	engine := &enginemocks.API{}
	engineParams := resourcetypes.Resources{"cpumem": {"cpu": 2}}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, "workloadid").Return(
		&types.Workload{ID: "workloadid", EngineParams: engineParams, Engine: engine}, nil,
	).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, "workloadid", engineParams).
		Return(types.ErrEngineNotImplemented).Once()

	c.wal.Recover(context.Background())
	store.AssertExpectations(t)
	engine.AssertExpectations(t)

	c.wal.Recover(context.Background())
}

func TestHandleCreateLambda(t *testing.T) {
	c := NewTestCluster()
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
	rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{}, nil)
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
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	c.wal.Recover(context.Background())
	time.Sleep(500 * time.Millisecond)
	c.wal.Recover(context.Background())
	time.Sleep(500 * time.Millisecond)
	store.AssertExpectations(t)
	eng.AssertExpectations(t)
}

func TestHandleCreateLambdaKeepsEntryUntilRemoved(t *testing.T) {
	c := NewTestCluster()
	enableTestWAL(t, c)

	_, err := c.wal.Log(eventCreateLambda, "workloadid")
	require.NoError(t, err)

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, "workloadid").Return(nil, types.ErrMockError).Twice()
	store.On("NotFound", types.ErrMockError).Return(false).Twice()

	c.wal.Recover(context.Background())
	time.Sleep(500 * time.Millisecond)
	c.wal.Recover(context.Background())
	time.Sleep(500 * time.Millisecond)
	store.AssertExpectations(t)
}

func enableTestWAL(t *testing.T, c *Calcium) {
	mockWALStore(c.store.(*storemocks.Store))
	journal, err := enableWAL(context.Background(), c.config, c, c.store)
	require.NoError(t, err)
	c.wal = journal
}

// mockWALStore backs the journal's three store calls with one map, so entries survive a Log.
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
}
