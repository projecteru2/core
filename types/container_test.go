package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/stretchr/testify/mock"
)

func TestContainerInspect(t *testing.T) {
	mockEngine := &mocks.API{}
	r := &enginetypes.VirtualizationInfo{ID: "12345"}
	mockEngine.On("VirtualizationInspect", mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(r, nil)

	ctx := context.Background()
	c := Container{}
	_, err := c.Inspect(ctx)
	assert.Error(t, err)
	c.Engine = mockEngine
	r2, _ := c.Inspect(ctx)
	assert.Equal(t, r.ID, r2.ID)
}

func TestContainerControl(t *testing.T) {
	mockEngine := &mocks.API{}
	mockEngine.On("VirtualizationStart", mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(nil)
	mockEngine.On("VirtualizationStop", mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(nil)
	mockEngine.On("VirtualizationRemove", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	ctx := context.Background()
	c := Container{}
	err := c.Start(ctx)
	assert.Error(t, err)
	err = c.Stop(ctx, 5*time.Second)
	assert.Error(t, err)
	err = c.Remove(ctx)
	assert.Error(t, err)

	c.Engine = mockEngine
	err = c.Start(ctx)
	assert.NoError(t, err)
	err = c.Stop(ctx, 5*time.Second)
	assert.NoError(t, err)
	err = c.Remove(ctx)
	assert.NoError(t, err)
}
