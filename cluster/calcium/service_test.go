package calcium

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestServiceStatusStream(t *testing.T) {
	c := NewTestCluster()
	c.config.Bind = ":5001"
	c.config.GRPCConfig.ServiceHeartbeatInterval = 100 * time.Millisecond
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 10 * time.Second
	store := &storemocks.Store{}
	c.store = store

	registered := map[string]int{}
	store.On("RegisterService", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("string"), mock.AnythingOfType("time.Duration")).Return(
		func(_ context.Context, addr string, _ time.Duration) error {
			if v, ok := registered[addr]; ok {
				registered[addr] = v + 1
			} else {
				registered[addr] = 1
			}
			return nil
		},
	)
	store.On("UnregisterService", mock.AnythingOfType("*context.timerCtx"), mock.AnythingOfType("string")).Return(
		func(_ context.Context, addr string) error {
			delete(registered, addr)
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	unregister, err := c.RegisterService(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(registered), 1)
	for _, v := range registered {
		assert.Equal(t, v, 1)
	}
	time.Sleep(100 * time.Millisecond)
	for _, v := range registered {
		assert.Equal(t, v, 2)
	}
	unregister()
	assert.Equal(t, len(registered), 0)
}

func TestWatchServiceStatus(t *testing.T) {
	c := NewTestCluster()
	c.config.GRPCConfig.ServiceDiscoveryPushInterval = 500 * time.Millisecond
	store := &storemocks.Store{}
	c.store = store
	c.watcher = &serviceWatcher{}

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
