package types

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/assert"

	enginetypes "github.com/docker/docker/api/types"
	enginemocks "github.com/projecteru2/core/3rdmocks"
)

func TestNode(t *testing.T) {
	mockEngine := &enginemocks.APIClient{}
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
	_, err = getEndpointHost(node.Endpoint)
	assert.Error(t, err)

	node.Endpoint = "tcp://1.1.1.1:1"
	ip = node.GetIP()
	assert.Equal(t, ip, "1.1.1.1")
	_, err = getEndpointHost(node.Endpoint)
	assert.NoError(t, err)

	node.Endpoint = "tcp://1.1.1.1"
	ip = node.GetIP()
	assert.Empty(t, ip)
	_, err = getEndpointHost(node.Endpoint)
	assert.Error(t, err)

	node.CPUUsage = 0.0
	node.SetCPUUsage(1.0, IncrUsage)
	assert.Equal(t, node.CPUUsage, 1.0)
	node.SetCPUUsage(1.0, DecrUsage)
	assert.Equal(t, node.CPUUsage, 0.0)
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

func TestRound(t *testing.T) {
	f := func(f float64) string {
		return strconv.FormatFloat(f, 'f', -1, 64)
	}
	a := 0.0199999998
	assert.Equal(t, f(Round(a)), "0.02")
	a = 0.1999998
	assert.Equal(t, f(Round(a)), "0.2")
	a = 1.999998
	assert.Equal(t, f(Round(a)), "2")
	a = 19.99998
	assert.Equal(t, f(Round(a)), "20")
}
