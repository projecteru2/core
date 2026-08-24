package calcium

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestRawEngine(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	engine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil).Once()
	_, err := c.RawEngine(ctx, &types.RawEngineOptions{ID: "id1", Op: "xxxx"})
	assert.NoError(t, err)
}

func TestRawEngineIgnoreLock(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	engine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil).Once()
	_, err := c.RawEngine(ctx, &types.RawEngineOptions{ID: "id1", Op: "xxxx", IgnoreLock: true})
	assert.NoError(t, err)
	engine.AssertExpectations(t)
}

func TestRawEngineRunsInlineWhenPoolRejects(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	pool, err := utils.NewPool(1)
	assert.NoError(t, err)
	c.pool = pool

	block := make(chan struct{})
	defer close(block)
	assert.NoError(t, pool.Invoke(func() { <-block }))

	store := c.store.(*storemocks.Store)
	engine := &enginemocks.API{}
	workload := &types.Workload{ID: "id1", Engine: engine}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	engine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil).Once()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, rawErr := c.RawEngine(ctx, &types.RawEngineOptions{ID: "id1", Op: "xxxx", IgnoreLock: true})
		assert.NoError(t, rawErr)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RawEngine hung when the pool rejected the task")
	}
	engine.AssertExpectations(t)
}
