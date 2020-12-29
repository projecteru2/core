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

func TestAddNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: name},
	}

	store := &storemocks.Store{}
	store.On("AddNode",
		mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(node, nil)
	c.store = store

	// fail by validating
	_, err := c.AddNode(ctx, &types.AddNodeOptions{})
	assert.Error(t, err)

	n, err := c.AddNode(ctx, &types.AddNodeOptions{
		Nodename: "nodename",
		Podname:  "podname",
		Endpoint: "endpoint",
	})
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestRemoveNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	// fail by validating
	assert.Error(t, c.RemoveNode(ctx, ""))

	name := "test"
	node := &types.Node{NodeMeta: types.NodeMeta{Name: name}}
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	store.On("GetNode",
		mock.Anything,
		mock.Anything).Return(node, nil)
	// fail, ListNodeWorkloads fail
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{}, types.ErrNoETCD).Once()
	assert.Error(t, c.RemoveNode(ctx, name))
	// fail, node still has associated workloads
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	assert.Error(t, c.RemoveNode(ctx, name))

	// succeed
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{}, nil)
	store.On("RemoveNode", mock.Anything, mock.Anything).Return(nil)
	pod := &types.Pod{Name: name}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod, nil)

	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	assert.NoError(t, c.RemoveNode(ctx, name))
}

func TestListPodNodes(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name1 := "test1"
	name2 := "test2"
	nodes := []*types.Node{
		{NodeMeta: types.NodeMeta{Name: name1}, Available: true},
		{NodeMeta: types.NodeMeta{Name: name2}, Available: false},
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

func TestGetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	// fail by validating
	_, err := c.GetNode(ctx, "")
	assert.Error(t, err)

	name := "test"
	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: name},
	}

	store := &storemocks.Store{}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	c.store = store

	n, err := c.GetNode(ctx, name)
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestSetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	name := "test"
	node := &types.Node{NodeMeta: types.NodeMeta{Name: name}}
	node.Init()

	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	// fail by validating
	_, err := c.SetNode(ctx, &types.SetNodeOptions{Nodename: ""})
	assert.Error(t, err)
	// failed by get node
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrCannotGetEngine).Once()
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "xxxx"})
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by no node name
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test1", StatusOpt: 2})
	assert.Error(t, err)
	// failed by updatenode
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", StatusOpt: 2})
	assert.Error(t, err)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	// succ when node available
	n, err := c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", StatusOpt: 2})
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
	// not available
	// failed by list node workloads
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", StatusOpt: 0, WorkloadsDown: true})
	assert.Error(t, err)
	workloads := []*types.Workload{{Name: "wrong_name"}, {Name: "a_b_c"}}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("SetWorkloadStatus",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD)
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", StatusOpt: 0, WorkloadsDown: true})
	assert.NoError(t, err)
	// test modify
	setOpts := &types.SetNodeOptions{
		Nodename:  "test",
		StatusOpt: 1,
		Labels:    map[string]string{"some": "1"},
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
	assert.Equal(t, n.CPU["2"], int64(1))
	assert.Equal(t, n.InitCPU["2"], int64(9))
	assert.Equal(t, n.CPU["3"], int64(10))
	assert.Equal(t, n.InitCPU["3"], int64(10))
	assert.Equal(t, len(n.CPU), 2)
	assert.Equal(t, len(n.InitCPU), 2)
	// succ set volume
	n.Volume = types.VolumeMap{"/sda1": 10, "/sda2": 20}
	setOpts.DeltaCPU = nil
	setOpts.DeltaVolume = types.VolumeMap{"/sda0": 5, "/sda1": 0, "/sda2": -1}
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	_, ok = n.Volume["/sda1"]
	assert.False(t, ok)
	assert.Equal(t, n.Volume["/sda0"], int64(5))
	assert.Equal(t, n.Volume["/sda2"], int64(19))
}
