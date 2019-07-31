package calcium

import (
	"context"
	"testing"

	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDissociateContainer(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	c1 := &types.Container{
		Meta: types.Meta{
			ID: "c1",
		},
		Podname:  "p1",
		Memory:   5 * types.MByte,
		Quota:    0.9,
		CPU:      types.CPUMap{"2": 90},
		Nodename: "node1",
	}

	node1 := &types.Node{
		Name:     "node1",
		MemCap:   types.GByte,
		CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Endpoint: "http://1.1.1.1:1",
	}

	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.DissociateContainer(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetContainer", mock.Anything, "c1").Return(c1, nil)
	store.On("GetNode", mock.Anything, mock.Anything, "node1").Return(node1, nil)
	// failed by RemoveContainer
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	ch, err = c.DissociateContainer(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("RemoveContainer", mock.Anything, mock.Anything).Return(nil)
	// success
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err = c.DissociateContainer(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
}
