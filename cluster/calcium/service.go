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
	for attempt := 0; ; attempt++ {
		if expiry, unregisterService, err = c.store.RegisterService(ctx, c.serviceAddress, c.config.GRPCConfig.ServiceHeartbeatInterval); err == nil {
			break
		}
		if !errors.Is(err, types.ErrKeyExists) {
			logger.Error(ctx, err, "failed to first register service")
			return nil, err
		}
		if attempt == 0 {
			logger.Debugf(ctx, "service key exists: %+v", err)
		} else {
			logger.Warnf(ctx, "service key %s still taken after %d attempts: %+v", c.serviceAddress, attempt+1, err)
		}
		select {
		case <-time.After(time.Second):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
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
				ne, us, err := c.store.RegisterService(ctx, c.serviceAddress, c.config.GRPCConfig.ServiceHeartbeatInterval)
				if err == nil {
					expiry = ne
					unregisterService = us
					continue
				}
				logger.Error(ctx, err, "failed to re-register service")
				select {
				case <-time.After(c.config.GRPCConfig.ServiceHeartbeatInterval):
				case <-ctx.Done():
					return
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
