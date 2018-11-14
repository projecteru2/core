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

func TestCPUMap(t *testing.T) {
	cpuMap := CPUMap{"0": 50, "1": 70}
	total := cpuMap.Total()
	assert.Equal(t, total, 120)

	cpuMap.Add(CPUMap{"0": 20})
	assert.Equal(t, cpuMap["0"], 70)

	cpuMap.Sub(CPUMap{"1": 20})
	assert.Equal(t, cpuMap["1"], 50)
}

func TestGetEndpointHost(t *testing.T) {
	endpoint := "xxxxx"
	s, err := GetEndpointHost(endpoint)
	assert.Error(t, err)
	assert.Empty(t, s)

	endpoint = "tcp://ip"
	s, err = GetEndpointHost(endpoint)
	assert.Error(t, err)
	assert.Empty(t, s)

	endpoint = "tcp://ip:port"
	s, err = GetEndpointHost(endpoint)
	assert.NoError(t, err)
	assert.NotEmpty(t, s)
}
