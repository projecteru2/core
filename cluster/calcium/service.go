package calcium

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

const (
	watchPushInterval = 10 * time.Second
)

type serviceWatcher struct {
	once sync.Once
	subs sync.Map
}

func (w *serviceWatcher) Start(s store.Store) {
	w.once.Do(func() {
		w.start(s)
	})
}

func (w *serviceWatcher) start(s store.Store) {
	ch, err := s.ServiceStatusStream(context.Background())
	if err != nil {
		log.Errorf("[WatchServiceStatus] failed to start watch: %v", err)
		return
	}

	go func() {
		defer log.Error("[WatchServiceStatus] goroutine exited")
		var (
			latestStatus types.ServiceStatus
			timer        *time.Timer = time.NewTimer(watchPushInterval / 2)
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
					Interval:  watchPushInterval,
				}
				w.dispatch(latestStatus)

			case <-timer.C:
				w.dispatch(latestStatus)
			}
			timer.Stop()
			timer.Reset(watchPushInterval / 2)
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

func (c *Calcium) WatchServiceStatus(ctx context.Context) (<-chan types.ServiceStatus, error) {
	ch := make(chan types.ServiceStatus)
	c.watcher.Start(c.store)
	id := c.watcher.Subscribe(ch)
	go func() {
		<-ctx.Done()
		c.watcher.Unsubscribe(id)
		close(ch)
	}()
	return ch, nil
}

func (c *Calcium) RegisterService(ctx context.Context) error {
	expire := 10 * time.Second
	timeoutCtx, cancel := context.WithTimeout(ctx, c.config.GlobalTimeout)
	defer cancel()
	if err := c.store.RegisterService(timeoutCtx, expire); err != nil {
		log.Errorf("[RegisterService] failed to register service: %v", err)
		return err
	}
	go func() {
		timer := time.NewTicker(expire / 2)
		for {
			select {
			case <-timer.C:
				if err := c.store.RegisterService(ctx, expire); err != nil {
					log.Errorf("[RegisterService] failed to register service: %v", err)
				}
			case <-ctx.Done():
				log.Infof("[RegisterService] context done: %v", ctx.Err())
				return
			}
		}
	}()
	return nil
}

func (c *Calcium) UnregisterService() error {
	c.cancel()
	return c.store.UnregisterService(context.Background())
}
