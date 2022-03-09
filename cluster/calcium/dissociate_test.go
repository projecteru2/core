package calcium

import (
	"context"
	"testing"
	"time"

	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDissociateWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	c1 := &types.Workload{
		ResourceMeta: types.ResourceMeta{
			MemoryLimit:     5 * int64(units.MiB),
			MemoryRequest:   5 * int64(units.MiB),
			CPUQuotaLimit:   0.9,
			CPUQuotaRequest: 0.9,
			CPU:             types.CPUMap{"2": 90},
		},
		ID:       "c1",
		Podname:  "p1",
		Nodename: "node1",
	}

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "node1",
			MemCap:   units.GiB,
			CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
			Endpoint: "http://1.1.1.1:1",
		},
	}

	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{c1}, nil)
	// failed by lock
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.DissociateWorkload(ctx, []string{"c1"})
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.AssertExpectations(t)

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by RemoveWorkload
	store.On("UpdateNodeResource", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD)
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
