package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRealloc(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	c.config.Scheduler.ShareBase = 100
	//	plugin := c.resource.GetPlugins()[0].(*resourcemocks.Plugin)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "node1",
			Endpoint: "http://1.1.1.1:1",
		},
		Engine: engine,
	}

	newC1 := func(context.Context, []string) []*types.Workload {
		return []*types.Workload{
			{
				ID:           "c1",
				Podname:      "p1",
				Engine:       engine,
				ResourceArgs: types.ResourceMeta{},
				Nodename:     "node1",
			},
		}
	}

	store.On("GetWorkload", mock.Anything, "c1").Return(newC1(context.TODO(), nil)[0], nil)
	opts := &types.ReallocOptions{
		ID:           "c1",
		ResourceOpts: types.WorkloadResourceOpts{},
	}

	// failed by GetNode
	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrNoETCD).Once()
	err := c.ReallocResource(ctx, opts)
	assert.EqualError(t, err, "ETCD must be set")
	store.AssertExpectations(t)
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	//	plugin.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodeResourceInfoResponse{
	//		ResourceInfo: &resources.NodeResourceInfo{
	//			Capacity: types.NodeResourceArgs{},
	//			Usage:    types.NodeResourceArgs{},
	//		},
	//	}, nil)

	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err = c.ReallocResource(ctx, opts)
	assert.EqualError(t, err, "ETCD must be set")
	store.AssertExpectations(t)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, []string{"c1"}).Return(newC1, nil)

	// failed by plugin
	//	plugin.On("GetReallocArgs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, context.DeadlineExceeded).Once()
	//	err = c.ReallocResource(ctx, opts)
	//	assert.Error(t, err)
	//	plugin.On("GetReallocArgs", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetReallocArgsResponse{}, nil)
	//	plugin.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&resources.SetNodeResourceUsageResponse{
	//		Before: types.NodeResourceArgs{},
	//		After:  types.NodeResourceArgs{},
	//	}, nil)

	// failed by UpdateWorkload
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	err = c.ReallocResource(ctx, opts)
	assert.EqualError(t, err, "ETCD must be set")
	store.AssertExpectations(t)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil)

	// failed by virtualization update resource
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNilEngine).Once()
	err = c.ReallocResource(ctx, opts)
	assert.ErrorIs(t, err, types.ErrNilEngine)
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// success
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD)
	err = c.ReallocResource(ctx, opts)
	assert.Nil(t, err)
}
