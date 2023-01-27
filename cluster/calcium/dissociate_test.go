package calcium

import (
	"context"
	"testing"
	"time"

	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDissociateWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	c1 := &types.Workload{
		Resources: &types.Resources{},
		ID:        "c1",
		Podname:   "p1",
		Nodename:  "node1",
	}

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "node1",
			Endpoint: "http://1.1.1.1:1",
		},
	}

	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{c1}, nil)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		nil, nil, nil, nil,
	)
	// failed by lock
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.DissociateWorkload(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.AssertExpectations(t)

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by RemoveWorkload
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&types.Resources{},
		&types.Resources{},
		nil,
	)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(types.ErrMockError).Once()
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	ch, err = c.DissociateWorkload(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	time.Sleep(time.Second)
	store.AssertExpectations(t)

	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)

	// success
	ch, err = c.DissociateWorkload(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
	store.AssertExpectations(t)
}
