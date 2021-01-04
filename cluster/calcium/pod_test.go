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

	store := &storemocks.Store{}
	store.On("AddPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pod, nil)
	c.store = store

	p, err := c.AddPod(ctx, name, "")
	assert.NoError(t, err)
	assert.Equal(t, p.Name, name)
}

func TestRemovePod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	assert.Error(t, c.RemovePod(ctx, ""))

	store := &storemocks.Store{}
	c.store = store
	store.On("RemovePod", mock.Anything, mock.Anything).Return(nil)
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}, Available: true}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.store = store
	assert.NoError(t, c.RemovePod(ctx, "podname"))
}

func TestListPods(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	pods := []*types.Pod{
		{Name: name},
	}

	store := &storemocks.Store{}
	store.On("GetAllPods", mock.Anything).Return(pods, nil)
	c.store = store

	ps, err := c.ListPods(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(ps), 1)
	assert.Equal(t, ps[0].Name, name)
}

func TestGetPod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	_, err := c.GetPod(ctx, "")
	assert.Error(t, err)

	name := "test"
	pod := &types.Pod{Name: name}
	store := &storemocks.Store{}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod, nil)
	c.store = store

	p, err := c.GetPod(ctx, name)
	assert.NoError(t, err)
	assert.Equal(t, p.Name, name)
}
