package types

import (
	"context"
	"strings"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNodeMeta(t *testing.T) {
	nm := NodeMeta{Name: "1"}
	nnm, err := nm.DeepCopy()
	assert.NoError(t, err)
	assert.Equal(t, nm.Name, nnm.Name)
}

func TestNodeInfo(t *testing.T) {
	mockEngine := &enginemocks.API{}
	r := &enginetypes.Info{ID: "test"}
	mockEngine.On("Info", mock.Anything).Return(r, ErrNoOps).Once()

	node := &Node{}
	ctx := context.Background()

	node.Engine = mockEngine
	err := node.Info(ctx)
	assert.Error(t, err)
	mockEngine.On("Info", mock.Anything).Return(r, nil)
	err = node.Info(ctx)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(node.NodeInfo, "test"))

	node.Bypass = true
	assert.True(t, node.IsDown())
}

func TestNodeMetrics(t *testing.T) {
	nm := Node{}
	assert.NotNil(t, nm.Metrics())
}
