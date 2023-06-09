package calcium

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/cockroachdb/errors"
)

// WatchServiceStatus returns chan of available service address
func (c *Calcium) WatchServiceStatus(ctx context.Context) (<-chan types.ServiceStatus, error) {
	id, ch := c.watcher.Subscribe(ctx)
	_ = c.pool.Invoke(func() {
		<-ctx.Done()
		c.watcher.Unsubscribe(id)
	})
	return ch, nil
}

// RegisterService writes self service address in store
func (c *Calcium) RegisterService(ctx context.Context) (unregister func(), err error) {
	serviceAddress, err := utils.GetOutboundAddress(c.config.Bind, c.config.ProbeTarget)
	logger := log.WithFunc("calcium.RegisterService")
	if err != nil {
		logger.Error(ctx, err, "failed to get outbound address")
		return nil, err
	}

	var (
		expiry            <-chan struct{}
		unregisterService func()
	)
	for {
		if expiry, unregisterService, err = c.registerService(ctx, serviceAddress); err == nil {
			break
		}
		if errors.Is(err, types.ErrKeyExists) {
			logger.Debugf(ctx, "service key exists: %+v", err)
			time.Sleep(time.Second)
			continue
		}
		logger.Error(ctx, err, "failed to first register service")
		return nil, err
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(ctx)
	_ = c.pool.Invoke(func() {
		defer func() {
			unregisterService()
			wg.Done()
		}()

		for {
			select {
			case <-expiry:
				// The original one had been expired, we're going to register again.
				if ne, us, err := c.registerService(ctx, serviceAddress); err != nil {
					logger.Error(ctx, err, "failed to re-register service")
					time.Sleep(c.config.GRPCConfig.ServiceHeartbeatInterval)
				} else {
					expiry = ne
					unregisterService = us
				}

			case <-ctx.Done():
				logger.Infof(ctx, "heartbeat done: %+v", ctx.Err())
				return
			}
		}
	})
	return func() {
		cancel()
		wg.Wait()
	}, nil
}

func (c *Calcium) registerService(ctx context.Context, addr string) (<-chan struct{}, func(), error) {
	return c.store.RegisterService(ctx, addr, c.config.GRPCConfig.ServiceHeartbeatInterval)
}
