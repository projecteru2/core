package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestCopy(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()

	_, err := c.Copy(ctx, &types.CopyOptions{
		Targets: map[string][]string{},
	})
	assert.Error(t, err)

	opts := &types.CopyOptions{
		Targets: map[string][]string{
			"cid": {
				"path1",
				"path2",
			},
		},
	}
	store := &storemocks.Store{}
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	c.store = store
	// failed by GetWorkload
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	workload := &types.Workload{ID: "cid"}
	workloads := []*types.Workload{workload}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(workloads, nil)
	// failed by VirtualizationCopyFrom
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(nil, "", types.ErrNilEngine).Twice()
	ch, err = c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(nil, "omg", nil)
	// success
	ch, err = c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
}
