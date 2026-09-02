package helium

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestHelium(t *testing.T) {
	chAddr := make(chan []string)

	store := &storemocks.Store{}
	store.On("ServiceStatusStream", mock.Anything).Return(chAddr, nil)

	grpcConfig := types.GRPCConfig{
		ServiceDiscoveryPushInterval: time.Duration(1) * time.Second,
	}
	service := New(t.Context(), grpcConfig, store)
	ID, chStatus := service.Subscribe()

	addresses1 := []string{
		"10.0.0.1",
		"10.0.0.2",
	}
	addresses2 := []string{
		"10.0.0.1",
	}

	chAddr <- addresses1
	status1 := <-chStatus
	chAddr <- addresses2
	status2 := <-chStatus
	assert.Equal(t, addresses1, status1.Addresses)
	assert.Equal(t, addresses2, status2.Addresses)
	assert.NotEqual(t, status1.Addresses, status2.Addresses)

	service.Unsubscribe(ID)
	close(chAddr)
}

func TestSubscribeGetsTheLatestStatusAtOnce(t *testing.T) {
	chAddr := make(chan []string)
	store := &storemocks.Store{}
	store.On("ServiceStatusStream", mock.Anything).Return(chAddr, nil)
	service := New(t.Context(), types.GRPCConfig{ServiceDiscoveryPushInterval: time.Hour}, store)

	chAddr <- []string{"10.0.0.1"}
	first, chFirst := service.Subscribe()
	<-chFirst
	service.Unsubscribe(first)

	ID, chStatus := service.Subscribe()
	select {
	case status := <-chStatus:
		assert.Equal(t, []string{"10.0.0.1"}, status.Addresses)
	case <-time.After(time.Second):
		t.Fatal("a new subscriber must be handed the latest status without waiting for the next push")
	}
	service.Unsubscribe(ID)
	close(chAddr)
}

func TestDispatchDoesNotWaitForAStuckSubscriber(t *testing.T) {
	chAddr := make(chan []string)
	store := &storemocks.Store{}
	store.On("ServiceStatusStream", mock.Anything).Return(chAddr, nil)
	service := New(t.Context(), types.GRPCConfig{ServiceDiscoveryPushInterval: time.Second}, store)
	stuckID, _ := service.Subscribe()
	readerID, reader := service.Subscribe()

	for range 3 {
		chAddr <- []string{"10.0.0.1"}
		chAddr <- []string{"10.0.0.2"}
		deadline := time.After(5 * time.Second)
		for latest := []string(nil); !slices.Equal(latest, []string{"10.0.0.2"}); {
			select {
			case status := <-reader:
				latest = status.Addresses
			case <-deadline:
				t.Fatal("a subscriber that never reads held up the others")
			}
		}
	}

	service.Unsubscribe(stuckID)
	service.Unsubscribe(readerID)
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

	for range 1000 {
		go func() {
			ID, _ := service.Subscribe()
			time.Sleep(time.Second)
			service.Unsubscribe(ID)
		}()
	}

	go func() {
		for range 1000 {
			chAddr <- []string{"hhh", "hhh2"}
		}
	}()

	time.Sleep(5 * time.Second)
}

func TestUnsubscribeAfterWatchClosed(t *testing.T) {
	chAddr := make(chan []string)

	store := &storemocks.Store{}
	store.On("ServiceStatusStream", mock.Anything).Return(chAddr, nil)

	grpcConfig := types.GRPCConfig{ServiceDiscoveryPushInterval: time.Second}
	service := New(t.Context(), grpcConfig, store)
	ID, _ := service.Subscribe()

	close(chAddr)
	<-service.done

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		service.Unsubscribe(ID)
	}()

	select {
	case <-returned:
	case <-time.After(5 * time.Second):
		assert.Fail(t, "Unsubscribe blocked after the watch loop exited")
	}
}
