package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestSetContainersStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	// failed
	store.On("GetContainer", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(nil, types.ErrBadCount).Once()
	err := c.SetContainersStatus(ctx, map[string][]byte{"123": []byte{}}, nil)
	assert.Error(t, err)
	container := &types.Container{
		ID:   "123",
		Name: "a_b_c",
	}
	store.On("GetContainer", mock.AnythingOfType("*context.emptyCtx"), mock.Anything).Return(container, nil)
	// failed by SetContainerStatus
	store.On("SetContainerStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(types.ErrBadCount).Once()
	err = c.SetContainersStatus(ctx, map[string][]byte{"123": []byte{}}, nil)
	assert.Error(t, err)
	// success
	store.On("SetContainerStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	err = c.SetContainersStatus(ctx, map[string][]byte{"123": []byte{}}, nil)
	assert.NoError(t, err)
}

func TestDeployStatusStream(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	dataCh := make(chan *types.DeployStatus)
	store := c.store.(*storemocks.Store)

	store.On("WatchDeployStatus", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything).Return(dataCh)
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
