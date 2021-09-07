package helium

import (
	"testing"
	"time"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestHelium(t *testing.T) {
	chAddr := make(chan []string)

	store := &storemocks.Store{}
	store.On("ServiceStatusStream", mock.Anything).Return(chAddr, nil)

	grpcConfig := types.GRPCConfig{
		ServiceDiscoveryPushInterval: time.Duration(1) * time.Second,
	}
	service := New(grpcConfig, store)
	chStatus := make(chan types.ServiceStatus)
	uuid := service.Subscribe(chStatus)

	addresses1 := []string{
		"10.0.0.1",
		"10.0.0.2",
	}
	addresses2 := []string{
		"10.0.0.1",
	}

	go func() {
		chAddr <- addresses1
		chAddr <- addresses2
	}()

	status1 := <-chStatus
	status2 := <-chStatus
	assert.Equal(t, addresses1, status1.Addresses)
	assert.Equal(t, addresses2, status2.Addresses)
	assert.NotEqual(t, status1.Addresses, status2.Addresses)

	service.Unsubscribe(uuid)
	close(chAddr)
	close(chStatus)
}
