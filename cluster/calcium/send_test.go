package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestSend(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	_, err := c.Send(ctx, &types.SendOptions{IDs: []string{}, Files: []types.LinuxFile{{Content: []byte("xxx")}}})
	assert.Error(t, err)
	_, err = c.Send(ctx, &types.SendOptions{IDs: []string{"id"}})
	assert.Error(t, err)

	opts := &types.SendOptions{
		IDs: []string{"cid"},
		Files: []types.LinuxFile{
			{
				Filename: "/tmp/1",
				Content:  []byte{},
			},
		},
	}
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine := &enginemocks.API{}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(
		[]*types.Workload{{ID: "cid", Engine: engine}}, nil,
	)
	engine.On("VirtualizationCopyChunkTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	).Return(types.ErrMockError).Once()
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationCopyChunkTo",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything,
	).Return(nil)
	ch, err = c.Send(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
		assert.Equal(t, r.ID, "cid")
		assert.Equal(t, r.Path, "/tmp/1")
	}
}
