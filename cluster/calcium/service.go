package calcium

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

type serviceWatcher struct {
	once sync.Once
	subs sync.Map
}

func (w *serviceWatcher) Start(s store.Store, pushInterval time.Duration) {
	w.once.Do(func() {
		w.start(s, pushInterval)
	})
}

func (w *serviceWatcher) start(s store.Store, pushInterval time.Duration) {
	ch, err := s.ServiceStatusStream(context.Background())
	if err != nil {
		log.Errorf("[WatchServiceStatus] failed to start watch: %v", err)
		return
	}

	go func() {
		defer log.Error("[WatchServiceStatus] goroutine exited")
		var (
			latestStatus types.ServiceStatus
			timer        *time.Timer = time.NewTimer(pushInterval)
		)
		for {
			select {
			case addresses, ok := <-ch:
				if !ok {
					log.Error("[WatchServiceStatus] watch channel closed")
					return
				}

				latestStatus = types.ServiceStatus{
					Addresses: addresses,
					Interval:  pushInterval * 2,
				}
				w.dispatch(latestStatus)

			case <-timer.C:
				w.dispatch(latestStatus)
			}
			timer.Stop()
			timer.Reset(pushInterval)
		}
	}()
}

func (w *serviceWatcher) dispatch(status types.ServiceStatus) {
	w.subs.Range(func(k, v interface{}) bool {
		c, ok := v.(chan<- types.ServiceStatus)
		if !ok {
			log.Error("[WatchServiceStatus] failed to cast channel from map")
			return true
		}
		c <- status
		return true
	})
}

func (w *serviceWatcher) Subscribe(ch chan<- types.ServiceStatus) uuid.UUID {
	id := uuid.New()
	_, _ = w.subs.LoadOrStore(id, ch)
	return id
}

func (w *serviceWatcher) Unsubscribe(id uuid.UUID) {
	w.subs.Delete(id)
}

// WatchServiceStatus returns chan of available service address
func (c *Calcium) WatchServiceStatus(ctx context.Context) (<-chan types.ServiceStatus, error) {
	ch := make(chan types.ServiceStatus)
	c.watcher.Start(c.store, c.config.GRPCConfig.ServiceDiscoveryPushInterval)
	id := c.watcher.Subscribe(ch)
	go func() {
		<-ctx.Done()
		c.watcher.Unsubscribe(id)
		close(ch)
	}()
	return ch, nil
}

// RegisterService writes self service address in store
func (c *Calcium) RegisterService(ctx context.Context) (unregister func(), err error) {
	serviceAddress, err := utils.GetOutboundAddress(c.config.Bind)
	if err != nil {
		log.Errorf("[RegisterService] failed to get outbound address: %v", err)
		return
	}
	if err = c.registerService(ctx, serviceAddress); err != nil {
		log.Errorf("[RegisterService] failed to first register service: %v", err)
		return
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		defer wg.Done()
		defer c.unregisterService(ctx, serviceAddress) // nolint

		timer := time.NewTicker(c.config.GRPCConfig.ServiceHeartbeatInterval / 2)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				_ = c.registerService(ctx, serviceAddress)
			case <-ctx.Done():
				log.Infof("[RegisterService] heartbeat done: %v", ctx.Err())
				return
			}
		}
	}()
	return func() {
		cancel()
		wg.Wait()
	}, err
}

func (c *Calcium) registerService(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.GRPCConfig.ServiceHeartbeatInterval)
	defer cancel()
	return c.store.RegisterService(ctx, addr, c.config.GRPCConfig.ServiceHeartbeatInterval)
}

func (c *Calcium) unregisterService(ctx context.Context, addr string) error {
	ctx, cancel := context.WithTimeout(ctx, c.config.GRPCConfig.ServiceHeartbeatInterval)
	defer cancel()
	return c.store.UnregisterService(ctx, addr)
}
