package calcium

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/projecteru2/core/engine/factory"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestAddNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	factory.InitEngineCache(ctx, c.config)

	opts := &types.AddNodeOptions{}
	// failed by validating
	_, err := c.AddNode(ctx, opts)
	assert.Error(t, err)

	nodename := "nodename"
	podname := "podname"
	opts.Nodename = nodename
	opts.Podname = podname
	opts.Endpoint = fmt.Sprintf("mock://%s", nodename)

	// failed by rmgr.AddNode
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("AddNode", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.AddNode(ctx, opts)
	assert.Error(t, err)
	rmgr.AssertExpectations(t)
	rmgr.On("AddNode", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, nil)
	rmgr.On("RemoveNode", mock.Anything, mock.Anything).Return(nil)
	// failed by store.AddNode
	store := c.store.(*storemocks.Store)
	store.On("AddNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.AddNode(ctx, opts)
	assert.Error(t, err)
	store.AssertExpectations(t)
	name := "test"
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     name,
			Endpoint: "endpoint",
		},
	}
	store.On("AddNode", mock.Anything, mock.Anything).Return(node, nil)
	rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{}, nil)
	n, err := c.AddNode(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, n.Name, name)
}

func TestRemoveNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	name := "test"
	node := &types.Node{NodeMeta: types.NodeMeta{Name: name}}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)

	// fail, ListNodeWorkloads fail
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{}, types.ErrMockError).Once()
	assert.Error(t, c.RemoveNode(ctx, name))

	// fail, node still has associated workloads
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{{}}, nil).Once()
	assert.Error(t, c.RemoveNode(ctx, name))
	store.AssertExpectations(t)

	// fail by store.RemoveNode
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return([]*types.Workload{}, nil)
	store.On("RemoveNode", mock.Anything, mock.Anything).Return(types.ErrMockError).Once()
	assert.Error(t, c.RemoveNode(ctx, name))

	// success
	store.On("RemoveNode", mock.Anything, mock.Anything).Return(nil)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("RemoveNode", mock.Anything, mock.Anything).Return(nil)
	assert.NoError(t, c.RemoveNode(ctx, name))
	store.AssertExpectations(t)
	rmgr.AssertExpectations(t)
}

func TestListPodNodes(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	opts := &types.ListNodesOptions{}
	store := c.store.(*storemocks.Store)
	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.ListPodNodes(ctx, opts)
	assert.Error(t, err)
	store.AssertExpectations(t)

	// success
	engine := &enginemocks.API{}
	engine.On("Info", mock.Anything).Return(nil, types.ErrMockError)
	name1 := "test1"
	name2 := "test2"
	nodes := []*types.Node{
		{NodeMeta: types.NodeMeta{Name: name1}, Engine: engine, Available: true},
		{NodeMeta: types.NodeMeta{Name: name2}, Engine: engine, Available: false},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nodes, nil)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, types.ErrMockError)
	opts.CallInfo = true

	ns, err := c.ListPodNodes(ctx, &types.ListNodesOptions{})
	assert.NoError(t, err)
	cnt := 0
	for range ns {
		cnt++
	}
	assert.Equal(t, cnt, 2)
	rmgr.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestGetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	// fail by validating
	_, err := c.GetNode(ctx, "")
	assert.Error(t, err)
	nodename := "test"

	// failed by store.GetNode
	store := c.store.(*storemocks.Store)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.GetNode(ctx, nodename)
	assert.Error(t, err)

	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: nodename},
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)

	// failed by GetNodeResourceInfo
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, types.ErrMockError).Once()
	_, err = c.GetNode(ctx, nodename)
	assert.Error(t, err)

	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

	// success
	node, err = c.GetNode(ctx, nodename)
	assert.NoError(t, err)
	assert.Equal(t, node.Name, nodename)

	rmgr.AssertExpectations(t)
	store.AssertExpectations(t)
}

func TestGetNodeEngine(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	// fail by validating
	_, err := c.GetNodeEngineInfo(ctx, "")
	assert.Error(t, err)
	nodename := "test"

	// fail by store.GetNode
	store := c.store.(*storemocks.Store)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.GetNode(ctx, nodename)
	assert.Error(t, err)

	// success
	engine := &enginemocks.API{}
	engine.On("Info", mock.Anything).Return(&enginetypes.Info{Type: "fake"}, nil)
	node := &types.Node{
		NodeMeta: types.NodeMeta{Name: nodename},
		Engine:   engine,
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)

	e, err := c.GetNodeEngineInfo(ctx, nodename)
	assert.NoError(t, err)
	assert.Equal(t, e.Type, "fake")
	store.AssertExpectations(t)
}

func TestSetNode(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	opts := &types.SetNodeOptions{}

	// failed by validating
	_, err := c.SetNode(ctx, opts)
	assert.Error(t, err)

	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	name := "test"
	opts.Nodename = name
	node := &types.Node{NodeMeta: types.NodeMeta{Name: name}}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)

	// failed by GetNodeResourceInfo
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, types.ErrMockError).Once()
	_, err = c.SetNode(ctx, opts)
	assert.Error(t, err)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)

	// for bypass
	opts.Bypass = types.TriTrue
	opts.WorkloadsDown = true
	// for setAllWorkloadsOnNodeDown
	workloads := []*types.Workload{{ID: "1", Name: "wrong_name"}, {ID: "2", Name: "a_b_c"}}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("SetWorkloadStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// for set endpoint
	endpoint := "mock://test2"
	opts.Endpoint = endpoint

	// for labels update
	labels := map[string]string{"a": "1", "b": "2"}
	opts.Labels = labels

	// failed by SetNodeResourceCapacity
	opts.Resources = resourcetypes.Resources{"a": {"a": 1}}
	rmgr.On("SetNodeResourceCapacity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil, nil, types.ErrMockError,
	).Once()
	_, err = c.SetNode(ctx, opts)
	assert.Error(t, err)
	rmgr.On("SetNodeResourceCapacity", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil, nil, nil,
	)

	// rollback
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(types.ErrMockError).Once()
	_, err = c.SetNode(ctx, opts)
	assert.Error(t, err)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{}, nil)

	// done
	node, err = c.SetNode(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, node.Endpoint, endpoint)
	assert.Equal(t, labels["a"], node.Labels["a"])
	store.AssertExpectations(t)
	time.Sleep(100 * time.Millisecond) // for send metrics testing
	rmgr.AssertExpectations(t)
}

func TestFilterNodes(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	// failed by GetNode
	nf := &types.NodeFilter{Includes: []string{"test"}}
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.filterNodes(ctx, nf)
	assert.Error(t, err)

	// succ by GetNode
	node1 := &types.Node{NodeMeta: types.NodeMeta{Name: "0"}}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	ns, err := c.filterNodes(ctx, nf)
	assert.NoError(t, err)
	assert.Len(t, ns, 1)

	// failed by GetNodesByPod
	nf.Includes = []string{}
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err = c.filterNodes(ctx, nf)
	assert.Error(t, err)

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
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nodes, nil)

	// no excludes
	ns, err = c.filterNodes(ctx, nf)
	assert.NoError(t, err)
	assert.Len(t, ns, 4)

	// excludes
	nf.Excludes = []string{"A", "C"}
	ns, err = c.filterNodes(ctx, nf)
	assert.NoError(t, err)
	assert.Len(t, ns, 2)
}
