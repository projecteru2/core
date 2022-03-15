package calcium

import (
	"context"
	"testing"
	"time"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRemoveWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store := c.store.(*storemocks.Store)

	// failed by GetWorkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.RemoveWorkload(ctx, []string{"xx"}, false)
	assert.True(t, errors.Is(err, types.ErrNoETCD))
	store.AssertExpectations(t)

	// failed by GetNode
	workload := &types.Workload{
		ID:       "xx",
		Name:     "test",
		Nodename: "test",
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	store.AssertExpectations(t)

	// failed by Remove
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
		},
	}
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Twice()
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD)
	ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	assert.NoError(t, c.doRemoveWorkloadSync(ctx, []string{"xx"}))
	time.Sleep(time.Second)
	store.AssertExpectations(t)

	// success
	engine := &enginemocks.API{}
	workload.Engine = engine
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.True(t, r.Success)
	}
	store.AssertExpectations(t)
}
