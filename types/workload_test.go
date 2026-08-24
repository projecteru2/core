package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
)

func TestWorkloadInspect(t *testing.T) {
	mockEngine := &mocks.API{}
	r := &enginetypes.VirtualizationInfo{ID: "12345"}
	mockEngine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(r, nil)

	c := Workload{Engine: mockEngine}
	r2, err := c.Inspect(t.Context())
	assert.NoError(t, err)
	assert.Equal(t, r.ID, r2.ID)
}

func TestWorkloadControl(t *testing.T) {
	mockEngine := &mocks.API{}
	mockEngine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationSuspend", mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationResume", mock.Anything, mock.Anything).Return(nil)

	ctx := t.Context()
	c := Workload{Engine: mockEngine}
	assert.NoError(t, c.Start(ctx))
	assert.NoError(t, c.Stop(ctx, true))
	assert.NoError(t, c.Remove(ctx, true))
	assert.NoError(t, c.Suspend(ctx))
	assert.NoError(t, c.Resume(ctx))
}

func TestRawEngine(t *testing.T) {
	mockEngine := &mocks.API{}
	mockEngine.On("RawEngine", mock.Anything, mock.Anything).Return(&enginetypes.RawEngineResult{}, nil)

	c := Workload{Engine: mockEngine}
	_, err := c.RawEngine(t.Context(), &RawEngineOptions{})
	assert.NoError(t, err)
}
