package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeployStatusStream(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	dataCh := make(chan *types.DeployStatus)

	store := &storemocks.Store{}
	store.On("WatchDeployStatus", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return(dataCh)
	c.store = store

	ID := "wtf"
	go func() {
		msg := &types.DeployStatus{
			ID: ID,
		}
		dataCh <- msg
		close(dataCh)
	}()

	ch := c.DeployStatusStream(ctx, "", "", "")
	for c := range ch {
		assert.Equal(t, c.ID, ID)
	}
}
