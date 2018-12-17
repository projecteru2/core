package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestAddPod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	pod := &types.Pod{
		Name: name,
	}

	store := &storemocks.Store{}
	store.On("AddPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(pod, nil)
	c.store = store

	p, err := c.AddPod(ctx, "", "", "")
	assert.NoError(t, err)
	assert.Equal(t, p.Name, name)
}

func TestAddNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	node := &types.Node{
		Name: name,
	}

	store := &storemocks.Store{}
	store.On("AddNode",
		mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything).Return(node, nil)
	c.store = store

	n, err := c.AddNode(ctx, "", "", "", "", "", "", 0, 0, int64(0), nil)
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestRemovePod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	store := &storemocks.Store{}
	store.On("RemovePod", mock.Anything, mock.Anything).Return(nil)
	c.store = store
	assert.NoError(t, c.RemovePod(ctx, ""))
}

func TestRemoveNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	name := "test"
	node := &types.Node{Name: name}
	store := &storemocks.Store{}
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(node, nil)
	store.On("DeleteNode", mock.Anything, mock.Anything).Return(nil)
	pod := &types.Pod{Name: name}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod, nil)
	c.store = store

	p, err := c.RemoveNode(ctx, "", "")
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

	store := &storemocks.Store{}
	store.On("GetAllPods", mock.Anything).Return(pods, nil)
	c.store = store

	ps, err := c.ListPods(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(ps), 1)
	assert.Equal(t, ps[0].Name, name)
}

func TestListPodNodes(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name1 := "test1"
	name2 := "test2"
	nodes := []*types.Node{
		{Name: name1, Available: true},
		{Name: name2, Available: false},
	}

	store := &storemocks.Store{}
	c.store = store
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Once()
	_, err := c.ListPodNodes(ctx, "", false)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nodes, nil)
	ns, err := c.ListPodNodes(ctx, "", false)
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 1)
	assert.Equal(t, ns[0].Name, name1)
	ns, err = c.ListPodNodes(ctx, "", true)
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 2)
}

func TestListContainers(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	containers := []*types.Container{
		{ID: ID},
	}

	store := &storemocks.Store{}
	c.store = store
	store.On("ListContainers", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	store.On("ListNodeContainers", mock.Anything, mock.Anything).Return(containers, nil)

	cs, err := c.ListContainers(ctx, &types.ListContainersOptions{Appname: "", Entrypoint: "", Nodename: ""})
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
	cs, err = c.ListNodeContainers(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, len(cs), 1)
	assert.Equal(t, cs[0].ID, ID)
}

func TestGetPod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	name := "test"
	pod := &types.Pod{Name: name}
	store := &storemocks.Store{}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod, nil)
	c.store = store

	p, err := c.GetPod(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, p.Name, name)
}

func TestGetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	node := &types.Node{
		Name: name,
	}

	store := &storemocks.Store{}
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(node, nil)
	store.On("GetNodeByName",
		mock.Anything,
		mock.Anything).Return(node, nil)
	c.store = store

	n, err := c.GetNode(ctx, "", "")
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
	n, err = c.GetNodeByName(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
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

func TestSetNodeAvailable(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	node := &types.Node{Name: name}

	store := &storemocks.Store{}
	c.store = store
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrCannotGetEngine).Once()
	_, err := c.SetNodeAvailable(ctx, "", "", true)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node, nil)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	_, err = c.SetNodeAvailable(ctx, "", "", true)
	assert.Error(t, err)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	n, err := c.SetNodeAvailable(ctx, "", "", true)
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestContainerDeployed(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	store := &storemocks.Store{}
	store.On("ContainerDeployed",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil)
	c.store = store

	assert.NoError(t, c.ContainerDeployed(ctx, "", "", "", "", ""))
}
