package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestExecuteContainer(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	// failed by GetContainer
	store.On("GetContainer", mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	ch := c.ExecuteContainer(ctx, &types.ExecuteContainerOptions{}, nil)
	for ac := range ch {
		assert.NotEmpty(t, ac.Data)
	}
	engine := &enginemocks.API{}
	ID := "abc"
	container := &types.Container{
		ID:     ID,
		Engine: engine,
	}
	store.On("GetContainer", mock.Anything, mock.Anything).Return(container, nil)
	// failed by Execute
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return(ID, nil, nil, types.ErrCannotGetEngine).Once()
	ch = c.ExecuteContainer(ctx, &types.ExecuteContainerOptions{}, nil)
	for ac := range ch {
		assert.Equal(t, ac.ContainerID, ID)
	}
}
