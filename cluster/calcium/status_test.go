package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestGetWorkloadsStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	cs := &types.StatusMeta{}

	// failed
	store.On("GetWorkloadStatus", mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err := c.GetWorkloadsStatus(ctx, []string{"a"})
	assert.Error(t, err)
	store.On("GetWorkloadStatus", mock.Anything, mock.Anything).Return(cs, nil)
	// succ
	r, err := c.GetWorkloadsStatus(ctx, []string{"a"})
	assert.NoError(t, err)
	assert.Len(t, r, 1)
}

func TestSetWorkloadsStatus(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	// failed
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	_, err := c.SetWorkloadsStatus(ctx, []*types.StatusMeta{{ID: "123"}}, nil)
	assert.Error(t, err)
	workload := &types.Workload{
		ID:   "123",
		Name: "a_b_c",
	}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	// failed by SetWorkloadStatus
	store.On("SetWorkloadStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(types.ErrBadCount).Once()
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
}

func TestWorkloadStatusStream(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	dataCh := make(chan *types.WorkloadStatus)
	store := c.store.(*storemocks.Store)

	store.On("WorkloadStatusStream", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(dataCh)
	go func() {
		msg := &types.WorkloadStatus{
			Delete: true,
		}
		dataCh <- msg
		close(dataCh)
	}()

	ch := c.WorkloadStatusStream(ctx, "", "", "", nil)
	for c := range ch {
		assert.Equal(t, c.Delete, true)
	}
}

func TestSetNodeStatus(t *testing.T) {
	assert := assert.New(t)
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "ep",
		},
	}
	// failed
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrBadCount).Once()
	assert.Error(c.SetNodeStatus(ctx, node.Name, 10))
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by SetWorkloadStatus
	store.On("SetNodeStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(types.ErrBadCount).Once()
	assert.Error(c.SetNodeStatus(ctx, node.Name, 10))
	// success
	store.On("SetNodeStatus",
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(nil)
	assert.NoError(c.SetNodeStatus(ctx, node.Name, 10))
}

func TestNodeStatusStream(t *testing.T) {
	assert := assert.New(t)
	c := NewTestCluster()
	ctx := context.Background()
	dataCh := make(chan *types.NodeStatus)
	store := c.store.(*storemocks.Store)

	store.On("NodeStatusStream", mock.Anything).Return(dataCh)
	go func() {
		msg := &types.NodeStatus{
			Alive: true,
		}
		dataCh <- msg
		close(dataCh)
	}()

	ch := c.NodeStatusStream(ctx)
	for c := range ch {
		assert.True(c.Alive)
	}
}
