package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestCopy(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

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
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	workload := &types.Workload{ID: "cid"}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, 0, int64(0), types.ErrMockError).Twice()
	ch, err = c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationCopyFrom", mock.Anything, mock.Anything, mock.Anything).Return([]byte("omg"), 0, 0, int64(0), nil)
	ch, err = c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
}
