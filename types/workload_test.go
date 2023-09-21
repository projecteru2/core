package types

import (
	"context"
	"testing"

	"github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestWorkloadInspect(t *testing.T) {
	mockEngine := &mocks.API{}
	r := &enginetypes.VirtualizationInfo{ID: "12345"}
	mockEngine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(r, nil)

	ctx := context.Background()
	c := Workload{}
	_, err := c.Inspect(ctx)
	assert.Error(t, err)
	c.Engine = mockEngine
	r2, _ := c.Inspect(ctx)
	assert.Equal(t, r.ID, r2.ID)
}

func TestWorkloadControl(t *testing.T) {
	mockEngine := &mocks.API{}
	mockEngine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationSuspend", mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationResume", mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	c := Workload{}
	assert.Error(t, c.Start(ctx))
	assert.Error(t, c.Stop(ctx, true))
	assert.Error(t, c.Remove(ctx, true))
	assert.Error(t, c.Suspend(ctx))
	assert.Error(t, c.Resume(ctx))

	c.Engine = mockEngine
	err := c.Start(ctx)
	assert.NoError(t, err)
	err = c.Stop(ctx, true)
	assert.NoError(t, err)
	err = c.Remove(ctx, true)
	assert.NoError(t, err)
	err = c.Suspend(ctx)
	assert.NoError(t, err)
	err = c.Resume(ctx)
	assert.NoError(t, err)
}

func TestRawEngine(t *testing.T) {
	mockEngine := &mocks.API{}
	mockEngine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil)

	ctx := context.Background()
	c := Workload{}
	_, err := c.RawEngine(ctx, &RawEngineOptions{})
	assert.Error(t, err)

	c.Engine = mockEngine
	_, err = c.RawEngine(ctx, &RawEngineOptions{})
	assert.NoError(t, err)
}
