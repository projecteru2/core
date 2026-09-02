package calcium

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/discovery/helium"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/utils"
)

func TestRegisterServiceDoesNotOccupyPool(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewTestCluster()
		c.pool.Release()
		pool, err := utils.NewPool(1)
		require.NoError(t, err)
		defer pool.Release()
		c.pool = pool

		store := c.store.(*storemocks.Store)
		store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).
			Return(make(<-chan struct{}), func() {}, nil).Once()
		unregister, err := c.RegisterService(t.Context())
		require.NoError(t, err)

		ran := make(chan struct{})
		invokeDone := make(chan error, 1)
		go func() {
			invokeDone <- pool.Invoke(func() { close(ran) })
		}()
		synctest.Wait()
		select {
		case <-ran:
		default:
			t.Error("service heartbeat occupied the pool")
		}

		unregister()
		synctest.Wait()
		require.NoError(t, <-invokeDone)
	})
}

func TestServiceStatusStream(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	store := c.store.(*storemocks.Store)

	var unregistered bool
	unregister := func() { unregistered = true }
	expiry := make(<-chan struct{})
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(expiry, unregister, nil).Once()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	unregisterService, err := c.RegisterService(ctx)
	assert.NoError(t, err)

	unregisterService()
	assert.True(t, unregistered)
}

func TestServiceStatusStreamWithMultipleRegisteringAsExpired(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	store := c.store.(*storemocks.Store)

	raw := make(chan struct{})
	var expiry <-chan struct{} = raw
	registeredAgain := make(chan struct{})
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(expiry, func() {}, nil).Once()
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Run(func(mock.Arguments) { close(registeredAgain) }).Return(make(<-chan struct{}), func() {}, nil).Once()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err := c.RegisterService(ctx)
	assert.NoError(t, err)

	close(raw)
	select {
	case <-registeredAgain:
	case <-time.After(5 * time.Second):
		t.Fatal("an expired registration must be renewed")
	}
	store.AssertExpectations(t)
}

func TestRegisterServiceFailed(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	store := c.store.(*storemocks.Store)

	experr := fmt.Errorf("error")
	store.On("RegisterService", mock.Anything, mock.Anything, mock.Anything).Return(make(<-chan struct{}), func() {}, experr).Once()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	_, err := c.RegisterService(ctx)
	assert.EqualError(t, err, "error")
}

func TestWatchServiceStatus(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 500 * time.Millisecond
	store := c.store.(*storemocks.Store)
	store.On("ServiceStatusStream", mock.Anything).Return(
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
	c.watcher = helium.New(t.Context(), c.config.GRPCConfig, c.store)

	ch, err := c.WatchServiceStatus(t.Context())
	assert.NoError(t, err)
	ch2, err := c.WatchServiceStatus(t.Context())
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
