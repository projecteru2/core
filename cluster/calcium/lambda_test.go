package calcium

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestRunAndWaitFailedThenWALCommitted(t *testing.T) {
	c, _ := newCreateWorkloadCluster(t)
	c.wal = &WAL{WAL: &walmocks.WAL{}}

	mwal := c.wal.WAL.(*walmocks.WAL)
	defer mwal.AssertExpectations(t)
	var walCommitted bool
	commit := wal.Commit(func() error {
		walCommitted = true
		return nil
	})
	mwal.On("Log", eventCreateLambda, mock.Anything).Return(commit, nil).Once()

	opts := &types.DeployOptions{
		Name:           "zc:name",
		Count:          2,
		DeployStrategy: strategy.Auto,
		Podname:        "p1",
		ResourceOpts:   types.ResourceOptions{CPUQuotaLimit: 1},
		Image:          "zc:test",
		Entrypoint: &types.Entrypoint{
			Name: "good-entrypoint",
		},
	}

	mstore := c.store.(*storemocks.Store)
	mstore.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("err")).Once()

	ch, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, walCommitted)
	require.Nil(t, <-ch) // recv nil due to the ch will be closed.

	lambdaID, exists := opts.Labels[labelLambdaID]
	require.True(t, exists)
	require.True(t, len(lambdaID) > 1)
	require.True(t, walCommitted)
}
