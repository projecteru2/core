package types

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/3rdmocks"
	"github.com/stretchr/testify/mock"
)

func TestContainerInspect(t *testing.T) {
	mockEngine := &mocks.APIClient{}
	r := enginetypes.ContainerJSON{}
	r.ContainerJSONBase = &enginetypes.ContainerJSONBase{ID: "12345"}
	mockEngine.On("ContainerInspect", mock.AnythingOfType("*context.timerCtx"), mock.Anything).Return(r, nil)

	ctx := context.Background()
	c := Container{}
	_, err := c.Inspect(ctx)
	assert.Error(t, err)
	c.Engine = mockEngine
	r2, err := c.Inspect(ctx)
	assert.Equal(t, r.ID, r2.ID)
}

func TestContainerControl(t *testing.T) {
	mockEngine := &mocks.APIClient{}
	mockEngine.On("ContainerStart", mock.AnythingOfType("*context.timerCtx"), mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("ContainerStop", mock.AnythingOfType("*context.timerCtx"), mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("ContainerRemove", mock.AnythingOfType("*context.emptyCtx"), mock.Anything, mock.Anything).Return(nil)

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
