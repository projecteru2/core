package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetContainersStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	cs := &types.StatusMeta{}

	// failed
	store.On("GetContainerStatus", mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err := c.GetContainersStatus(ctx, []string{"a"})
	assert.Error(t, err)
	store.On("GetContainerStatus", mock.Anything, mock.Anything).Return(cs, nil)
	// succ
	r, err := c.GetContainersStatus(ctx, []string{"a"})
	assert.NoError(t, err)
	assert.Len(t, r, 1)
}

func TestSetContainersStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	// failed
	store.On("GetContainer", mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err := c.SetContainersStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.Error(t, err)
	container := &types.Container{
		ID:   "123",
		Name: "a_b_c",
	}
	store.On("GetContainer", mock.Anything, mock.Anything).Return(container, nil)
	// failed by SetContainerStatus
	store.On("SetContainerStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(types.ErrBadCount).Once()
	_, err = c.SetContainersStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.Error(t, err)
	// success
	store.On("SetContainerStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	r, err := c.SetContainersStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.NoError(t, err)
	assert.Len(t, r, 1)
}

func TestContainerStatusStream(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	dataCh := make(chan *types.ContainerStatus)
	store := c.store.(*storemocks.Store)

	store.On("ContainerStatusStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(dataCh)
	go func() {
		msg := &types.ContainerStatus{
			Delete: true,
		}
		dataCh <- msg
		close(dataCh)
	}()

	ch := c.ContainerStatusStream(ctx, "", "", "", nil)
	for c := range ch {
		assert.Equal(t, c.Delete, true)
	}
}
