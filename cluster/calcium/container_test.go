package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestListContainers(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	containers := []*types.Container{
		{ID: ID},
	}

	store := &storemocks.Store{}
	c.store = store
	store.On("ListContainers", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)

	cs, err := c.ListContainers(ctx, &types.ListContainersOptions{Appname: "", Entrypoint: "", Nodename: ""})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
	cs, err = c.ListNodeContainers(ctx, "", nil)
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}

func TestGetContainers(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	container := &types.Container{ID: ID}
	containers := []*types.Container{container}

	store := &storemocks.Store{}
	c.store = store
	store.On("GetContainer", mock.Anything, mock.Anything).Return(container, nil)
	store.On("GetContainers", mock.Anything, mock.Anything).Return(containers, nil)

	savedContainer, err := c.GetContainer(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, savedContainer.ID, ID)
	cs, err := c.GetContainers(ctx, []string{})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}
