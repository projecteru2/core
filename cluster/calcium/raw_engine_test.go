package calcium

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestRawEngine(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	mockLocks(t, store)
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	engine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil).Once()
	_, err := c.RawEngine(ctx, &types.RawEngineOptions{ID: "id1", Op: "xxxx"})
	assert.NoError(t, err)
}

func TestRawEngineIgnoreLock(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	engine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil).Once()
	_, err := c.RawEngine(ctx, &types.RawEngineOptions{ID: "id1", Op: "xxxx", IgnoreLock: true})
	assert.NoError(t, err)
	engine.AssertExpectations(t)
}
