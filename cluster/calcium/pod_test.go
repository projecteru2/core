package calcium

import (
	"context"
	"testing"

	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAddPod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	_, err := c.AddPod(ctx, "", "")
	assert.Error(t, err)

	name := "test"
	pod := &types.Pod{
		Name: name,
	}

	store := c.store.(*storemocks.Store)
	store.On("AddPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pod, nil)

	p, err := c.AddPod(ctx, name, "")
	assert.NoError(t, err)
	assert.Equal(t, p.Name, name)
}

func TestRemovePod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	// failed by validating
	assert.Error(t, c.RemovePod(ctx, ""))

	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("RemovePod", mock.Anything, mock.Anything).Return(nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		[]*types.Node{{NodeMeta: types.NodeMeta{Name: "test"}}}, nil)

	assert.NoError(t, c.RemovePod(ctx, "podname"))
	store.AssertExpectations(t)
}

func TestGetPod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	_, err := c.GetPod(ctx, "")
	assert.Error(t, err)

	name := "test"
	pod := &types.Pod{Name: name}
	store := c.store.(*storemocks.Store)
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod, nil)

	p, err := c.GetPod(ctx, name)
	assert.NoError(t, err)
	assert.Equal(t, p.Name, name)
}

func TestListPods(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	name := "test"
	pods := []*types.Pod{
		{Name: name},
	}

	store := c.store.(*storemocks.Store)
	store.On("GetAllPods", mock.Anything).Return(pods, nil)

	ps, err := c.ListPods(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(ps), 1)
	assert.Equal(t, ps[0].Name, name)
}
