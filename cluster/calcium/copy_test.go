package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func TestCopy(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.CopyOptions{
		Targets: map[string][]string{
			"cid": {
				"path1",
				"path2",
			},
		},
	}
	store := &storemocks.Store{}
	c.store = store
	// failed by GetContainer
	store.On("GetContainer", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.Copy(ctx, opts)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	container := &types.Container{Meta: types.Meta{ID: "cid"}}
	engine := &enginemocks.API{}
	container.Engine = engine
	store.On("GetContainer", mock.Anything, mock.Anything).Return(container, nil)
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
