package calcium

import (
	"bytes"
	"context"
	"io/ioutil"
	"testing"

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
	container := &types.Container{
		ID:     ID,
		Engine: engine,
	}
	ctx := context.Background()
	opts := &types.LogStreamOptions{ID: ID}
	// failed by GetContainer
	store.On("GetContainer", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.Empty(t, c.Data)
	}
	store.On("GetContainer", mock.Anything, mock.Anything).Return(container, nil)
	// failed by VirtualizationLogs
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, types.ErrNodeExist).Once()
	ch, err = c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.Empty(t, c.Data)
	}
	reader := bytes.NewBufferString("aaaa\nbbbb\n")
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(ioutil.NopCloser(reader), nil)
	// success
	ch, err = c.LogStream(ctx, opts)
	assert.NoError(t, err)
	for c := range ch {
		assert.Equal(t, c.ID, ID)
		assert.NotEmpty(t, c.Data)
	}
}
