package calcium

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcetypes "github.com/projecteru2/core/resources"
	resourcemocks "github.com/projecteru2/core/resources/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPodResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	podname := "testpod"
	nodename := "testnode"
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.PodResource(ctx, podname)
	assert.Error(t, err)
	store.AssertExpectations(t)

	// failed by ListNodeWorkloads
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: nodename,
		},
	}
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		map[string]types.NodeResourceArgs{"test": map[string]interface{}{"abc": 123}},
		map[string]types.NodeResourceArgs{"test": map[string]interface{}{"abc": 123}},
		[]string{types.ErrNoETCD.Error()},
		nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	ch, err = c.PodResource(ctx, podname)
	assert.NoError(t, err)
	msg := <-ch
	assert.True(t, strings.Contains(msg.Diffs[0], types.ErrNoETCD.Error()))
	store.AssertExpectations(t)

	workloads := []*types.Workload{
		{ResourceArgs: map[string]types.WorkloadResourceArgs{}},
		{ResourceArgs: map[string]types.WorkloadResourceArgs{}},
	}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine

	// success
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, []string{"a"}, nil)
	r, err := c.PodResource(ctx, podname)
	assert.NoError(t, err)
	first := <-r
	assert.NotEmpty(t, first.Diffs)
}

func TestNodeResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	nodename := "testnode"
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil, nil, nil, nil,
	)
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: nodename,
		},
	}
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// fail by validating
	_, err := c.NodeResource(ctx, "", false)
	assert.Error(t, err)
	// failed by GetNode
	store.On("GetNode", ctx, nodename).Return(nil, types.ErrNoETCD).Once()
	_, err = c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, nodename).Return(node, nil)
	// failed by list node workloads
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	workloads := []*types.Workload{
		{
			ResourceArgs: map[string]types.WorkloadResourceArgs{},
		},
		{
			ResourceArgs: map[string]types.WorkloadResourceArgs{},
		},
	}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	rmgr.On("ConvertNodeResourceInfoToMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*resourcetypes.Metrics{}, nil)
	// success but workload inspect failed
	nr, err := c.NodeResource(ctx, nodename, true)
	time.Sleep(time.Second)
	assert.NoError(t, err)
	assert.Equal(t, nr.Name, nodename)
	assert.NotEmpty(t, nr.Diffs)
	details := strings.Join(nr.Diffs, ",")
	assert.Contains(t, details, "inspect failed")
}
