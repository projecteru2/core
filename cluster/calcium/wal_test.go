package calcium

import (
	"context"
	"fmt"
	"testing"
	"time"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandleCreateWorkloadNoHandle(t *testing.T) {
	c := NewTestCluster()
	wal, err := newCalciumWAL(c)
	require.NoError(t, err)
	c.wal = wal

	wrkid := "workload-id"
	_, err = c.wal.logCreateWorkload(wrkid, "nodename")
	require.NoError(t, err)

	wrk := &types.Workload{
		ID: "wrkid",
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, nil).Once()

	c.wal.Recover(context.TODO())

	// Recovers nothing.
	c.wal.Recover(context.TODO())
}

func TestHandleCreateWorkloadError(t *testing.T) {
	c := NewTestCluster()
	wal, err := newCalciumWAL(c)
	require.NoError(t, err)
	c.wal = wal

	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: "nodename"},
		Engine:   &enginemocks.API{},
	}
	wrkid := "workload-id"
	_, err = c.wal.logCreateWorkload(wrkid, node.Name)
	require.NoError(t, err)

	wrk := &types.Workload{
		ID:       wrkid,
		Nodename: node.Name,
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, fmt.Errorf("err")).Once()
	c.wal.Recover(context.TODO())

	err = types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("keys: [%s]", wrkid))
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, err)
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(nil, fmt.Errorf("err")).Once()
	c.wal.Recover(context.TODO())

	store.On("GetNode", mock.Anything, wrk.Nodename).Return(node, nil)
	eng, ok := node.Engine.(*enginemocks.API)
	require.True(t, ok)
	defer eng.AssertExpectations(t)
	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(fmt.Errorf("err")).
		Once()
	c.wal.Recover(context.TODO())

	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(fmt.Errorf("Error: No such container: %s", wrk.ID)).
		Once()
	c.wal.Recover(context.TODO())

	// Nothing recovered.
	c.wal.Recover(context.TODO())
}

func TestHandleCreateWorkloadHandled(t *testing.T) {
	c := NewTestCluster()
	wal, err := newCalciumWAL(c)
	require.NoError(t, err)
	c.wal = wal

	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: "nodename"},
		Engine:   &enginemocks.API{},
	}

	wrkid := "workload-id"
	_, err = c.wal.logCreateWorkload(wrkid, node.Name)
	require.NoError(t, err)

	wrk := &types.Workload{
		ID:       wrkid,
		Nodename: node.Name,
		Engine:   nil, // explicitly set a nil as it will be evaluated by CreateWorkloadHandler.Handle.
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)
	err = types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("keys: [%s]", wrkid))
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, err).Once()
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(node, nil)

	eng, ok := node.Engine.(*enginemocks.API)
	require.True(t, ok)
	defer eng.AssertExpectations(t)
	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(nil).
		Once()

	c.wal.Recover(context.TODO())

	// Recovers nothing.
	c.wal.Recover(context.TODO())
}

func TestHandleCreateLambda(t *testing.T) {
	c := NewTestCluster()
	wal, err := newCalciumWAL(c)
	require.NoError(t, err)
	c.wal = wal

	_, err = c.wal.logCreateLambda(&types.CreateWorkloadMessage{WorkloadID: "workloadid"})
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
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	c.wal.Recover(context.TODO())
	time.Sleep(500 * time.Millisecond)
	store.AssertExpectations(t)

	_, err = c.wal.logCreateLambda(&types.CreateWorkloadMessage{WorkloadID: "workloadid"})
	require.NoError(t, err)
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
	eng.On("VirtualizationResourceRemap", mock.Anything, mock.Anything).Return(nil, nil).Once()
	store.On("GetWorkloads", mock.Anything, []string{wrk.ID}).
		Return([]*types.Workload{wrk}, nil).
		Twice()
	store.On("RemoveWorkload", mock.Anything, wrk).
		Return(nil).
		Once()
	store.On("UpdateNodeResource", mock.Anything, node, mock.Anything, mock.Anything).
		Return(nil).
		Once()
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	c.wal.Recover(context.TODO())
	// Recovered nothing.
	c.wal.Recover(context.TODO())
	time.Sleep(500 * time.Millisecond)
	store.AssertExpectations(t)
	eng.AssertExpectations(t)
}
