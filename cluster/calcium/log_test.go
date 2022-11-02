package calcium

import (
	"bytes"
	"context"
	"io"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	ctx := context.Background()
	opts := &types.LogStreamOptions{ID: ID}
	// failed by GetWorkload
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	ch, err := c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.Empty(t, c.Data)
	}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	// failed by VirtualizationLogs
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, nil, types.ErrMockError).Once()
	ch, err = c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.Empty(t, c.Data)
	}
	reader := bytes.NewBufferString("aaaa\nbbbb\n")
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(io.NopCloser(reader), nil, nil)
	// success
	ch, err = c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.NotEmpty(t, c.Data)
	}
}
