package calcium

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestHandleCreateWorkloadNoHandle(t *testing.T) {
	c := NewTestCluster()
	wal, err := newCalciumWAL(c)
	require.NoError(t, err)
	c.wal = wal

	wrkid := "workload-id"
	_, err = c.wal.logCreateWorkload(context.Background(), wrkid, "nodename")
	require.NoError(t, err)

	wrk := &types.Workload{
		ID: "wrkid",
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, nil).Once()

	c.wal.Recover(context.Background())

	// Recovers nothing.
	c.wal.Recover(context.Background())
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
	_, err = c.wal.logCreateWorkload(context.Background(), wrkid, node.Name)
	require.NoError(t, err)

	wrk := &types.Workload{
		ID:       wrkid,
		Nodename: node.Name,
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, fmt.Errorf("err")).Once()
	c.wal.Recover(context.Background())

	err = types.NewDetailedErr(types.ErrBadCount, fmt.Sprintf("keys: [%s]", wrkid))
	store.On("GetWorkload", mock.Anything, wrkid).Return(wrk, err)
	store.On("GetNode", mock.Anything, wrk.Nodename).Return(nil, fmt.Errorf("err")).Once()
	c.wal.Recover(context.Background())

	store.On("GetNode", mock.Anything, wrk.Nodename).Return(node, nil)
	eng, ok := node.Engine.(*enginemocks.API)
	require.True(t, ok)
	defer eng.AssertExpectations(t)
	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(fmt.Errorf("err")).
		Once()
	c.wal.Recover(context.Background())

	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(fmt.Errorf("Error: No such container: %s", wrk.ID)).
		Once()
	c.wal.Recover(context.Background())

	// Nothing recovered.
	c.wal.Recover(context.Background())
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
	_, err = c.wal.logCreateWorkload(context.Background(), wrkid, node.Name)
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

	c.wal.Recover(context.Background())

	// Recovers nothing.
	c.wal.Recover(context.Background())
}

func TestHandleCreateLambda(t *testing.T) {
	c := NewTestCluster()
	wal, err := newCalciumWAL(c)
	require.NoError(t, err)
	c.wal = wal

	deployOpts := &types.DeployOptions{
		Name:       "appname",
		Entrypoint: &types.Entrypoint{Name: "entry"},
		Labels:     map[string]string{labelLambdaID: "lambda"},
	}
	_, err = c.wal.logCreateLambda(context.Background(), deployOpts)
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
	defer store.AssertExpectations(t)
	store.On("ListWorkloads", mock.Anything, deployOpts.Name, deployOpts.Entrypoint.Name, "", int64(0), deployOpts.Labels).
		Return(nil, fmt.Errorf("err")).
		Once()
	c.wal.Recover(context.Background())

	store.On("ListWorkloads", mock.Anything, deployOpts.Name, deployOpts.Entrypoint.Name, "", int64(0), deployOpts.Labels).
		Return([]*types.Workload{wrk}, nil).
		Once()
	store.On("GetWorkloads", mock.Anything, []string{wrk.ID}).
		Return([]*types.Workload{wrk}, nil).
		Once()
	store.On("GetNode", mock.Anything, wrk.Nodename).
		Return(node, nil)

	eng := wrk.Engine.(*enginemocks.API)
	defer eng.AssertExpectations(t)
	eng.On("VirtualizationRemove", mock.Anything, wrk.ID, true, true).
		Return(nil).
		Once()

	store.On("RemoveWorkload", mock.Anything, wrk).
		Return(nil).
		Once()
	store.On("UpdateNodeResource", mock.Anything, node, mock.Anything, mock.Anything).
		Return(nil).
		Once()

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	c.wal.Recover(context.Background())
	// Recovered nothing.
	c.wal.Recover(context.Background())
}
