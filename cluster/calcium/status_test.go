package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetNodeStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	ns := &types.NodeStatus{
		Nodename: "test",
		Podname:  "test",
		Alive:    true,
	}
	// failed
	store.On("GetNodeStatus", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.GetNodeStatus(ctx, "test")
	assert.Error(t, err)

	store.On("GetNodeStatus", mock.Anything, mock.Anything).Return(ns, nil)
	s, err := c.GetNodeStatus(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, s.Nodename, "test")
	assert.True(t, s.Alive)
	store.AssertExpectations(t)
}

func TestSetNodeStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "ep",
		},
	}
	// failed by GetNode
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	assert.Error(t, c.SetNodeStatus(ctx, node.Name, 10))
	// failed by SetWorkloadStatus
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	store.On("SetNodeStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(types.ErrMockError).Once()
	assert.Error(t, c.SetNodeStatus(ctx, node.Name, 10))
	// success
	store.On("SetNodeStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	assert.NoError(t, c.SetNodeStatus(ctx, node.Name, 10))
	store.AssertExpectations(t)
}

func TestNodeStatusStream(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	dataCh := make(chan *types.NodeStatus)
	store := c.store.(*storemocks.Store)
	store.On("NodeStatusStream", mock.Anything).Return(dataCh)

	go func() {
		defer close(dataCh)
		msg := &types.NodeStatus{
			Alive: true,
		}
		dataCh <- msg
	}()

	for c := range c.NodeStatusStream(ctx) {
		assert.True(t, c.Alive)
	}
	store.AssertExpectations(t)
}

func TestGetWorkloadsStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	cs := &types.StatusMeta{}

	// failed GetWorkloadStatus
	store.On("GetWorkloadStatus", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.GetWorkloadsStatus(ctx, []string{"a"})
	assert.Error(t, err)
	// success
	store.On("GetWorkloadStatus", mock.Anything, mock.Anything).Return(cs, nil)
	r, err := c.GetWorkloadsStatus(ctx, []string{"a"})
	assert.NoError(t, err)
	assert.Len(t, r, 1)
	store.AssertExpectations(t)
}

func TestSetWorkloadsStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	// no meta, generate by id
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	_, err := c.SetWorkloadsStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.Error(t, err)

	// failed by workload name
	workload := &types.Workload{
		ID:   "123",
		Name: "invalid",
	}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil).Once()
	_, err = c.SetWorkloadsStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.Error(t, err)

	// failed by SetworkloadStatus
	workload.Name = "a_b_c"
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("SetWorkloadStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(types.ErrMockError).Once()
	_, err = c.SetWorkloadsStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.Error(t, err)

	// success
	store.On("SetWorkloadStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	r, err := c.SetWorkloadsStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.NoError(t, err)
	assert.Len(t, r, 1)
	store.AssertExpectations(t)
}

func TestWorkloadStatusStream(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	dataCh := make(chan *types.WorkloadStatus)
	store := c.store.(*storemocks.Store)

	store.On("WorkloadStatusStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(dataCh)
	go func() {
		defer close(dataCh)
		msg := &types.WorkloadStatus{
			Delete: true,
		}
		dataCh <- msg
	}()

	for c := range c.WorkloadStatusStream(ctx, "", "", "", nil) {
		assert.Equal(t, c.Delete, true)
	}
}
