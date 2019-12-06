package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	lockmocks "github.com/projecteru2/core/lock/mocks"
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

	p, err := c.AddPod(ctx, "", "")
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
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(node, nil)
	c.store = store

	n, err := c.AddNode(ctx, "", "", "", "", "", "", 0, 0, int64(0), int64(0), nil, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestRemovePod(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	store.On("RemovePod", mock.Anything, mock.Anything).Return(nil)
	c.store = store
	node := &types.Node{Name: "n1", Available: true}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(node, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.store = store
	assert.NoError(t, c.RemovePod(ctx, ""))
}

func TestRemoveNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	name := "test"
	node := &types.Node{Name: name}
	store := &storemocks.Store{}
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(node, nil)
	store.On("RemoveNode", mock.Anything, mock.Anything).Return(nil)
	pod := &types.Pod{Name: name}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod, nil)
	c.store = store

	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	err := c.RemoveNode(ctx, "", "")
	assert.NoError(t, err)
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
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.ListPodNodes(ctx, "", nil, false)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	ns, err := c.ListPodNodes(ctx, "", nil, false)
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 2)
	assert.Equal(t, ns[0].Name, name1)
	ns, err = c.ListPodNodes(ctx, "", nil, true)
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 2)
}

func TestListContainers(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	ID := "testID"
	containers := []*types.Container{
		&types.Container{ID: ID},
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

func TestSetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	node := &types.Node{Name: name}

	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed by get node
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrCannotGetEngine).Once()
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	_, err := c.SetNode(ctx, &types.SetNodeOptions{})
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything, mock.Anything).Return(node, nil)
	// failed by updatenode
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Status: 2})
	assert.Error(t, err)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	// succ when node available
	n, err := c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", Status: 2})
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
	// not available
	// failed by list node containers
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", Status: 0})
	assert.Error(t, err)
	containers := []*types.Container{&types.Container{Name: "wrong_name"}, &types.Container{Name: "a_b_c"}}
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	store.On("SetContainerStatus",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD)
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", Status: 0})
	assert.NoError(t, err)
	// test modify
	setOpts := &types.SetNodeOptions{
		Nodename: "test",
		Status:   1,
		Labels:   map[string]string{"some": "1"},
	}
	// set label
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.Labels["some"], "1")
	// set numa
	setOpts.NUMA = types.NUMA{"100": "node1"}
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.NUMA["100"], "node1")
	// failed by numa node memory < 0
	n.NUMAMemory = types.NUMAMemory{"node1": 1}
	n.InitNUMAMemory = types.NUMAMemory{"node1": 2}
	setOpts.DeltaNUMAMemory = types.NUMAMemory{"node1": -10}
	n, err = c.SetNode(ctx, setOpts)
	assert.Error(t, err)
	// succ set numa node memory
	n.NUMAMemory = types.NUMAMemory{"node1": 1}
	n.InitNUMAMemory = types.NUMAMemory{"node1": 2}
	setOpts.DeltaNUMAMemory = types.NUMAMemory{"node1": -1}
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.NUMAMemory["node1"], int64(0))
	setOpts.DeltaNUMAMemory = types.NUMAMemory{}
	// failed set storage
	n.StorageCap = 1
	n.InitStorageCap = 2
	setOpts.DeltaStorage = -10
	n, err = c.SetNode(ctx, setOpts)
	assert.Error(t, err)
	// succ set storage
	n.StorageCap = 1
	n.InitStorageCap = 2
	setOpts.DeltaStorage = -1
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.StorageCap, int64(0))
	setOpts.DeltaStorage = 0
	// failed set memory
	n.MemCap = 1
	n.InitMemCap = 2
	setOpts.DeltaMemory = -10
	n, err = c.SetNode(ctx, setOpts)
	assert.Error(t, err)
	// succ set storage
	n.MemCap = 1
	n.InitMemCap = 2
	setOpts.DeltaMemory = -1
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.MemCap, int64(0))
	setOpts.DeltaMemory = 0
	// failed by set cpu
	n.CPU = types.CPUMap{"1": 1}
	n.InitCPU = types.CPUMap{"1": 2}
	setOpts.DeltaCPU = types.CPUMap{"1": -10}
	n, err = c.SetNode(ctx, setOpts)
	assert.Error(t, err)
	// succ set cpu, add and del
	n.CPU = types.CPUMap{"1": 1, "2": 2}
	n.InitCPU = types.CPUMap{"1": 10, "2": 10}
	setOpts.DeltaCPU = types.CPUMap{"1": 0, "2": -1, "3": 10}
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	_, ok := n.CPU["1"]
	assert.False(t, ok)
	assert.Equal(t, n.CPU["2"], 1)
	assert.Equal(t, n.InitCPU["2"], 9)
	assert.Equal(t, n.CPU["3"], 10)
	assert.Equal(t, n.InitCPU["3"], 10)
	assert.Equal(t, len(n.CPU), 2)
	assert.Equal(t, len(n.InitCPU), 2)
}
