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

func TestNodeInfo(t *testing.T) {
	mockEngine := &enginemocks.API{}
	r := &enginetypes.Info{ID: "test"}
	mockEngine.On("Info", mock.Anything).Return(r, nil)

	node := &Node{}
	ctx := context.Background()

	node.Engine = mockEngine
	err := node.Info(ctx)
	assert.NoError(t, err)
	assert.True(t, strings.Contains(node.NodeInfo, "test"))
}
