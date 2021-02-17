package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestHandleCreateLambda(t *testing.T) {
	c := NewTestCluster()
	deployOpts := &types.DeployOptions{
		Name:       "appname",
		Entrypoint: &types.Entrypoint{Name: "entry"},
		Labels:     map[string]string{labelLambdaID: "lambda"},
	}
	_, err := c.wal.logCreateLambda(context.Background(), deployOpts)
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
