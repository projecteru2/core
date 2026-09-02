package calcium

import (
	"bytes"
	"context"
	"io"
	"testing"
	"testing/synctest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestLogStream(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	engine := &enginemocks.API{}
	ID := "test"
	workload := &types.Workload{
		ID:     ID,
		Engine: engine,
	}
	ctx := t.Context()
	opts := &types.LogStreamOptions{ID: ID}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.Empty(t, c.Data)
	}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, nil, types.ErrMockError).Once()
	ch, err = c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.Empty(t, c.Data)
	}
	reader := bytes.NewBufferString("aaaa\nbbbb\n")
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(io.NopCloser(reader), nil, nil)
	ch, err = c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.NotEmpty(t, c.Data)
	}
}

func TestLogStreamStopsWhenTheCallerLeaves(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewTestCluster()
		defer c.pool.Release()
		store := c.store.(*storemocks.Store)
		engine := &enginemocks.API{}
		store.On("GetWorkload", mock.Anything, mock.Anything).Return(&types.Workload{ID: "test", Engine: engine}, nil)
		engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(io.NopCloser(bytes.NewBufferString("aaaa\nbbbb\n")), nil, nil)
		ctx, cancel := context.WithCancel(t.Context())

		ch, err := c.LogStream(ctx, &types.LogStreamOptions{ID: "test"})
		assert.NoError(t, err)
		synctest.Wait()
		cancel()
		synctest.Wait()

		_, open := <-ch
		assert.False(t, open, "the stream must close once its reader is gone")
	})
}
