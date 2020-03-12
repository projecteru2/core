package calcium

import (
	"context"
	"testing"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCreateContainer(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	opts := &types.DeployOptions{}
	store := c.store.(*storemocks.Store)

	// failed by GetPod
	store.On("GetPod", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	store.On("GetPod", mock.Anything, mock.Anything).Return(&types.Pod{Name: "test"}, nil)
	// failed by count
	opts.Count = 0
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	opts.Count = 1

	// failed by memory check
	opts.Memory = -1
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
	opts.Memory = 1

	// failed by CPUQuota
	opts.CPUQuota = -1
	_, err = c.CreateContainer(ctx, opts)
	assert.Error(t, err)
}
