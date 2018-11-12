package types

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/3rdmocks"
)

func TestNode(t *testing.T) {
	mockEngine := &mocks.APIClient{}
	r := enginetypes.Info{ID: "test"}
	mockEngine.On("Info", mock.AnythingOfType("*context.timerCtx")).Return(r, nil)

	node := &Node{}
	ctx := context.Background()
	_, err := node.Info(ctx)
	assert.Error(t, err)

	node.Engine = mockEngine
	info, err := node.Info(ctx)
	assert.NoError(t, err)
	assert.Equal(t, info.ID, "test")

	ip := node.GetIP()
	assert.Empty(t, ip)
	_, err = GetEndpointHost(node.Endpoint)
	assert.Error(t, err)

	node.Endpoint = "tcp://1.1.1.1:1"
	ip = node.GetIP()
	assert.Equal(t, ip, "1.1.1.1")
	_, err = GetEndpointHost(node.Endpoint)
	assert.NoError(t, err)

	node.Endpoint = "tcp://1.1.1.1"
	ip = node.GetIP()
	assert.Empty(t, ip)
	_, err = GetEndpointHost(node.Endpoint)
	assert.Error(t, err)
}
