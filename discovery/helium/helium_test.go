package helium

import (
	"context"
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
	service := New(t.Context(), grpcConfig, store)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	uuid, chStatus := service.Subscribe(ctx)

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
}

func TestPanic(t *testing.T) {
	chAddr := make(chan []string)

	store := &storemocks.Store{}
	store.On("ServiceStatusStream", mock.Anything).Return(chAddr, nil)

	grpcConfig := types.GRPCConfig{
		ServiceDiscoveryPushInterval: time.Duration(1) * time.Second,
	}
	service := New(t.Context(), grpcConfig, store)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	for range 1000 {
		go func() {
			uuid, _ := service.Subscribe(ctx)
			time.Sleep(time.Second)
			service.Unsubscribe(uuid)
		}()
	}

	go func() {
		for range 1000 {
			chAddr <- []string{"hhh", "hhh2"}
		}
	}()

	time.Sleep(5 * time.Second)
}
