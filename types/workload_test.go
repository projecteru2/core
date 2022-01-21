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
	c.Engine = mockEngine
	r2, _ := c.Inspect(ctx)
	assert.Equal(t, r.ID, r2.ID)
}

func TestWorkloadControl(t *testing.T) {
	mockEngine := &mocks.API{}
	mockEngine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	mockEngine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(1, nil)

	ctx := context.Background()
	c := Workload{}
	c.Engine = mockEngine
	err := c.Start(ctx)
	assert.NoError(t, err)
	err = c.Stop(ctx, true)
	assert.NoError(t, err)
	err = c.Remove(ctx, true)
	assert.NoError(t, err)
}
