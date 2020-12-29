package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
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
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	// failed by GetWorkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.RemoveWorkload(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	workload := &types.Workload{
		ID:       "xx",
		Name:     "test",
		Nodename: "test",
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	// failed by GetNode
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
		},
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by Remove
	ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.False(t, r.Success)
	}
	assert.NoError(t, c.doRemoveWorkloadSync(ctx, []string{"xx"}))
	engine := &enginemocks.API{}
	workload.Engine = engine
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	// success
	ch, err = c.RemoveWorkload(ctx, []string{"xx"}, false, 0)
	assert.NoError(t, err)
	for r := range ch {
		assert.True(t, r.Success)
	}
}
