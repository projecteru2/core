package calico

import (
	"context"
	"errors"
	"testing"

	enginetypes "github.com/docker/docker/api/types"
	enginenetwork "github.com/docker/docker/api/types/network"
	enginemocks "github.com/projecteru2/core/3rdmocks"
	"github.com/projecteru2/core/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestConnectToNetwork(t *testing.T) {
	s := New()
	containerID := "1234567812345678123456781234567812345678123456781234567812345678"
	mockEngine := &enginemocks.APIClient{}
	// container id not 64
	err := s.ConnectToNetwork(context.Background(), "", "", "")
	assert.Error(t, err)
	// no engine
	err = s.ConnectToNetwork(context.Background(), containerID, "", "")
	assert.Error(t, err)

	ctx := utils.ContextWithDockerEngine(context.Background(), mockEngine)
	mockEngine.On("NetworkConnect",
		mock.AnythingOfType("*context.valueCtx"), mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	// no ipv4
	err = s.ConnectToNetwork(ctx, containerID, "", "")
	assert.NoError(t, err)
	// invaild ipv4
	err = s.ConnectToNetwork(ctx, containerID, "", "x.0.1.8")
	assert.Error(t, err)
	// vaild ipv4
	err = s.ConnectToNetwork(ctx, containerID, "", "127.0.0.1")
	assert.NoError(t, err)
}

func TestDisconnectFromNetwork(t *testing.T) {
	s := New()
	containerID := "1234567812345678123456781234567812345678123456781234567812345678"
	mockEngine := &enginemocks.APIClient{}
	// container id not 64
	err := s.ConnectToNetwork(context.Background(), "", "", "")
	assert.Error(t, err)
	// no engine
	err = s.ConnectToNetwork(context.Background(), containerID, "", "")
	assert.Error(t, err)
	ctx := utils.ContextWithDockerEngine(context.Background(), mockEngine)
	mockEngine.On("NetworkDisconnect",
		mock.AnythingOfType("*context.valueCtx"), mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	err = s.DisconnectFromNetwork(ctx, containerID, "")
	assert.NoError(t, err)
}

func TestListNetworks(t *testing.T) {
	s := New()
	mockEngine := &enginemocks.APIClient{}
	// no engine
	_, err := s.ListNetworks(context.Background(), "")
	assert.Error(t, err)
	ctx := utils.ContextWithDockerEngine(context.Background(), mockEngine)
	mockEngine.On("NetworkList",
		mock.AnythingOfType("*context.valueCtx"), mock.Anything,
	).Return(nil, errors.New("test")).Once()
	// List failed
	_, err = s.ListNetworks(ctx, "")
	assert.Error(t, err)
	// List
	networkName := "test"
	subnet := "10.2.0.0/16"
	result := []enginetypes.NetworkResource{
		enginetypes.NetworkResource{
			Name: networkName,
			IPAM: enginenetwork.IPAM{
				Config: []enginenetwork.IPAMConfig{
					enginenetwork.IPAMConfig{Subnet: subnet},
				}}},
	}
	mockEngine.On("NetworkList",
		mock.AnythingOfType("*context.valueCtx"), mock.Anything,
	).Return(result, nil)
	ns, err := s.ListNetworks(ctx, "")
	assert.NoError(t, err)
	assert.Equal(t, len(ns), 1)
	assert.Equal(t, ns[0].Name, networkName)
	assert.Equal(t, ns[0].Subnets[0], subnet)
}
