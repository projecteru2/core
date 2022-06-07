package calcium

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/discovery/helium"
	storemocks "github.com/projecteru2/core/store/mocks"
)

func TestServiceStatusStream(t *testing.T) {
	c := NewTestCluster()
	c.config.Bind = ":5001"
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 10 * time.Second
	store := &storemocks.Store{}
	c.store = store

	var unregistered bool
	unregister := func() { unregistered = true }
	expiry := make(<-chan struct{})
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(expiry, unregister, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unregisterService, err := c.RegisterService(ctx)
	assert.NoError(t, err)

	unregisterService()
	assert.True(t, unregistered)
}

func TestServiceStatusStreamWithMultipleRegisteringAsExpired(t *testing.T) {
	c := NewTestCluster()
	c.config.Bind = ":5001"
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 10 * time.Second
	store := &storemocks.Store{}
	c.store = store

	raw := make(chan struct{})
	var expiry <-chan struct{} = raw
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(expiry, func() {}, nil).Once()
	// Once the original one expired, the new calling's expiry must also be a brand new <-chan.
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(make(<-chan struct{}), func() {}, nil).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_, err := c.RegisterService(ctx)
	assert.NoError(t, err)

	// Triggers the original one expired.
	close(raw)
	// Waiting for the second calling of store.RegisterService.
	time.Sleep(time.Millisecond)
	store.AssertExpectations(t)
}

func TestRegisterServiceFailed(t *testing.T) {
	c := NewTestCluster()
	c.config.Bind = ":5001"
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 10 * time.Second
	store := &storemocks.Store{}
	c.store = store

	experr := fmt.Errorf("error")
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(make(<-chan struct{}), func() {}, experr).Once()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, err := c.RegisterService(ctx)
	assert.EqualError(t, err, "error")
}

func TestWatchServiceStatus(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 500 * time.Millisecond
	store := &storemocks.Store{}
	c.store = store
	store.On("ServiceStatusStream", mock.AnythingOfType("*context.emptyCtx")).Return(
		func(_ context.Context) chan []string {
			ch := make(chan []string)
			go func() {
				ticker := time.NewTicker(50 * time.Millisecond)
				cnt := 0
				for range ticker.C {
					if cnt == 2 {
						break
					}
					ch <- []string{fmt.Sprintf("127.0.0.1:500%d", cnt)}
					cnt++
				}
			}()
			return ch
		}, nil,
	)
	c.watcher = helium.New(c.config.GRPCConfig, c.store)

	ch, err := c.WatchServiceStatus(context.Background())
	assert.NoError(t, err)
	ch2, err := c.WatchServiceStatus(context.Background())
	assert.NoError(t, err)
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		defer wg.Done()
		assert.Equal(t, (<-ch).Addresses, []string{"127.0.0.1:5000"})
		assert.Equal(t, (<-ch).Addresses, []string{"127.0.0.1:5001"})
		assert.Equal(t, (<-ch).Addresses, []string{"127.0.0.1:5001"})
	}()
	go func() {
		defer wg.Done()
		assert.Equal(t, (<-ch2).Addresses, []string{"127.0.0.1:5000"})
		assert.Equal(t, (<-ch2).Addresses, []string{"127.0.0.1:5001"})
		assert.Equal(t, (<-ch2).Addresses, []string{"127.0.0.1:5001"})
	}()
	wg.Wait()
}
