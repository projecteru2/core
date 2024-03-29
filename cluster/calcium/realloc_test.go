package calcium

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
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

func TestRealloc(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{}, nil)
	c.config.Scheduler.ShareBase = 100

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
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
				ID:        "c1",
				Podname:   "p1",
				Engine:    engine,
				Resources: resourcetypes.Resources{},
				Nodename:  "node1",
			},
		}
	}

	store.On("GetWorkload", mock.Anything, "c1").Return(newC1(context.Background(), nil)[0], nil)
	opts := &types.ReallocOptions{
		ID:        "c1",
		Resources: resourcetypes.Resources{},
	}

	// failed by GetNode
	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrMockError).Once()
	err := c.ReallocResource(ctx, opts)
	assert.True(t, errors.Is(err, types.ErrMockError))
	store.AssertExpectations(t)
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)

	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.ReallocResource(ctx, opts)
	assert.True(t, errors.Is(err, types.ErrMockError))
	store.AssertExpectations(t)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, []string{"c1"}).Return(newC1, nil)

	// failed by plugin
	rmgr.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, nil, nil, types.ErrMockError,
	).Once()
	err = c.ReallocResource(ctx, opts)
	assert.Error(t, err)
	rmgr.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		nil,
	)
	rmgr.On("RollbackRealloc", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// failed by UpdateWorkload
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(types.ErrMockError).Once()
	err = c.ReallocResource(ctx, opts)
	assert.True(t, errors.Is(err, types.ErrMockError))
	store.AssertExpectations(t)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil)

	// failed by virtualization update resource
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNilEngine).Once()
	err = c.ReallocResource(ctx, opts)
	assert.ErrorIs(t, err, types.ErrNilEngine)
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// success
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	err = c.ReallocResource(ctx, opts)
	assert.Nil(t, err)
}
