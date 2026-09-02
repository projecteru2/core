package calcium

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) WatchServiceStatus(ctx context.Context) (<-chan types.ServiceStatus, error) {
	id, ch := c.watcher.Subscribe()
	context.AfterFunc(ctx, func() { c.watcher.Unsubscribe(id) })
	return ch, nil
}

// RegisterService registers this core's service address in the store.
func (c *Calcium) RegisterService(ctx context.Context) (unregister func(), err error) {
	logger := log.WithFunc("calcium.RegisterService")

	var (
		expiry            <-chan struct{}
		unregisterService func()
	)
	for {
		if expiry, unregisterService, err = c.store.RegisterService(ctx, c.serviceAddress, c.config.GRPCConfig.ServiceHeartbeatInterval); err == nil {
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

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	wg.Go(func() {
		defer log.SentryDefer()
		defer func() {
			unregisterService()
		}()

		for {
			select {
			case <-expiry:
				if ne, us, err := c.store.RegisterService(ctx, c.serviceAddress, c.config.GRPCConfig.ServiceHeartbeatInterval); err != nil {
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
