package calcium

import (
	"context"
	"testing"

	"github.com/projecteru2/core/engine/factory"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/resources"
	resourcemocks "github.com/projecteru2/core/resources/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAddNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	factory.InitEngineCache(ctx, c.config)
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	name := "test"
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     name,
			Endpoint: "endpoint",
		},
	}

	store := &storemocks.Store{}
	store.On("AddNode",
		mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything).Return(node, nil)
	c.store = store
	plugin.On("AddNode", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&resources.AddNodeResponse{}, nil)

	// fail by validating
	_, err := c.AddNode(ctx, &types.AddNodeOptions{})
	assert.Error(t, err)

	n, err := c.AddNode(ctx, &types.AddNodeOptions{
		Nodename: "nodename",
		Podname:  "podname",
		Endpoint: "mock://" + name,
	})
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestRemoveNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	name := "test"
	node := &types.Node{NodeMeta: types.NodeMeta{Name: name}}
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{
			Capacity: types.NodeResourceArgs{},
			Usage:    types.NodeResourceArgs{},
		},
	}, nil)
	plugin.On("RemoveNode", mock.Anything, mock.Anything).Return(&resources.RemoveNodeResponse{}, nil)

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
	_, err := c.ListPodNodes(ctx, &types.ListNodesOptions{})
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	ns, err := c.ListPodNodes(ctx, &types.ListNodesOptions{})
	nss := []*types.Node{}
	for n := range ns {
		nss = append(nss, n)
	}
	assert.NoError(t, err)
	assert.Equal(t, len(nss), 2)
	assert.Equal(t, nss[0].Name, name1)
	ns, err = c.ListPodNodes(ctx, &types.ListNodesOptions{})
	assert.NoError(t, err)
	cnt := 0
	for range ns {
		cnt++
	}
	assert.Equal(t, cnt, 2)
}

func TestGetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{
			Capacity: types.NodeResourceArgs{},
			Usage:    types.NodeResourceArgs{},
		},
	}, nil)

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

func TestGetNodeEngine(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	// fail by validating
	_, err := c.GetNodeEngine(ctx, "")
	assert.Error(t, err)

	engine := &enginemocks.API{}
	engine.On("Info", mock.Anything).Return(&enginetypes.Info{Type: "fake"}, nil)
	name := "test"
	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: name},
		Engine:   engine,
	}

	store := &storemocks.Store{}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	c.store = store

	e, err := c.GetNodeEngine(ctx, name)
	assert.NoError(t, err)
	assert.Equal(t, e.Type, "fake")
}

func TestSetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{
			Capacity: types.NodeResourceArgs{},
			Usage:    types.NodeResourceArgs{},
		},
	}, nil)
	plugin.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&resources.SetNodeResourceUsageResponse{
		Before: types.NodeResourceArgs{},
		After:  types.NodeResourceArgs{},
	}, nil)

	name := "test"
	node := &types.Node{NodeMeta: types.NodeMeta{Name: name}}

	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("SetNodeStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)

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
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test1"})
	assert.Error(t, err)
	// failed by updatenode
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(types.ErrCannotGetEngine).Once()
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test"})
	assert.Error(t, err)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	// succ when node available
	n, err := c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", Endpoint: "tcp://127.0.0.1:2379"})
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
	assert.Equal(t, n.Endpoint, "tcp://127.0.0.1:2379")
	// not available
	// can still set node even if ListNodeWorkloads fails
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", WorkloadsDown: true})
	assert.NoError(t, err)
	workloads := []*types.Workload{{Name: "wrong_name"}, {Name: "a_b_c"}}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("SetWorkloadStatus",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD)
	_, err = c.SetNode(ctx, &types.SetNodeOptions{Nodename: "test", WorkloadsDown: true})
	assert.NoError(t, err)
	// test modify
	setOpts := &types.SetNodeOptions{
		Nodename: "test",
		Labels:   map[string]string{"some": "1"},
	}
	// set label
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.Labels["some"], "1")
	// set endpoint
	setOpts.Endpoint = "tcp://10.10.10.10:2379"
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.Endpoint, "tcp://10.10.10.10:2379")
	// no impact on endpoint
	setOpts.Endpoint = ""
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
	assert.Equal(t, n.Endpoint, "tcp://10.10.10.10:2379")
	// set ca / cert / key
	setOpts.Ca = "hh"
	setOpts.Cert = "hh"
	setOpts.Key = "hh"
	n, err = c.SetNode(ctx, setOpts)
	assert.NoError(t, err)
}

func TestFilterNodes(t *testing.T) {
	assert := assert.New(t)
	c := NewTestCluster()
	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
		ResourceInfo: &resources.NodeResourceInfo{
			Capacity: types.NodeResourceArgs{},
			Usage:    types.NodeResourceArgs{},
		},
	}, nil)
	store := c.store.(*storemocks.Store)
	nodes := []*types.Node{
		{
			NodeMeta: types.NodeMeta{Name: "A"},
		},
		{
			NodeMeta: types.NodeMeta{Name: "B"},
		},
		{
			NodeMeta: types.NodeMeta{Name: "C"},
		},
		{
			NodeMeta: types.NodeMeta{Name: "D"},
		},
	}

	// error
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.New("fail to list pod nodes")).Once()
	_, err := c.filterNodes(context.Background(), types.NodeFilter{Includes: []string{}, Excludes: []string{"A", "X"}})
	assert.Error(err)

	// empty nodenames, non-empty excludeNodenames
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil).Once()
	nodes1, err := c.filterNodes(context.Background(), types.NodeFilter{Includes: []string{}, Excludes: []string{"A", "B"}})
	assert.NoError(err)
	assert.Equal("C", nodes1[0].Name)
	assert.Equal("D", nodes1[1].Name)

	// empty nodenames, empty excludeNodenames
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil).Once()
	nodes2, err := c.filterNodes(context.Background(), types.NodeFilter{Includes: []string{}, Excludes: []string{}})
	assert.NoError(err)
	assert.Equal(4, len(nodes2))

	// non-empty nodenames, empty excludeNodenames
	store.On("GetNode", mock.Anything, "O").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "O"}}, nil).Once()
	store.On("GetNode", mock.Anything, "P").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "P"}}, nil).Once()
	nodes3, err := c.filterNodes(context.Background(), types.NodeFilter{Includes: []string{"O", "P"}, Excludes: []string{}})
	assert.NoError(err)
	assert.Equal("O", nodes3[0].Name)
	assert.Equal("P", nodes3[1].Name)

	// non-empty nodenames
	store.On("GetNode", mock.Anything, "X").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "X"}}, nil).Once()
	store.On("GetNode", mock.Anything, "Y").Return(&types.Node{NodeMeta: types.NodeMeta{Name: "Y"}}, nil).Once()
	nodes4, err := c.filterNodes(context.Background(), types.NodeFilter{Includes: []string{"X", "Y"}, Excludes: []string{"A", "B"}})
	assert.NoError(err)
	assert.Equal("X", nodes4[0].Name)
	assert.Equal("Y", nodes4[1].Name)
}
