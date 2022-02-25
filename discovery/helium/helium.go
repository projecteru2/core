package helium

import (
	"context"
	"sync"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"

	"github.com/google/uuid"
)

// Helium .
type Helium struct {
	sync.Once
	lock   *sync.RWMutex
	config types.GRPCConfig
	stor   store.Store
	subs   sync.Map
}

// New .
func New(config types.GRPCConfig, stor store.Store) *Helium {
	h := &Helium{}
	h.config = config
	h.stor = stor
	h.lock = &sync.RWMutex{}
	h.Do(func() {
		h.start(context.TODO()) // TODO rewrite ctx here, because this will run only once!
	})
	return h
}

// Subscribe .
func (h *Helium) Subscribe(ch chan<- types.ServiceStatus) uuid.UUID {
	h.lock.Lock()
	defer h.lock.Unlock()
	id := uuid.New()
	_, _ = h.subs.LoadOrStore(id, ch)
	return id
}

// Unsubscribe .
func (h *Helium) Unsubscribe(id uuid.UUID) {
	h.lock.Lock()
	defer h.lock.Unlock()
	h.subs.Delete(id)
}

func (h *Helium) start(ctx context.Context) {
	ch, err := h.stor.ServiceStatusStream(ctx)
	if err != nil {
		log.Errorf(nil, "[WatchServiceStatus] failed to start watch: %v", err) //nolint
		return
	}

	go func() {
		log.Info("[WatchServiceStatus] service discovery start")
		defer log.Error("[WatchServiceStatus] service discovery exited")
		var latestStatus types.ServiceStatus
		timer := time.NewTimer(h.config.ServiceDiscoveryPushInterval)
		for {
			select {
			case addresses, ok := <-ch:
				if !ok {
					log.Error("[WatchServiceStatus] watch channel closed")
					return
				}

				latestStatus = types.ServiceStatus{
					Addresses: addresses,
					Interval:  h.config.ServiceDiscoveryPushInterval * 2,
				}
			case <-timer.C:
			}
			h.dispatch(latestStatus)
			timer.Stop()
			timer.Reset(h.config.ServiceDiscoveryPushInterval)
		}
	}()
}

func (h *Helium) dispatch(status types.ServiceStatus) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	h.subs.Range(func(k, v interface{}) bool {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf(context.TODO(), "[dispatch] dispatch %s failed, err: %v", k, err)
			}
		}()
		c, ok := v.(chan<- types.ServiceStatus)
		if !ok {
			log.Error("[WatchServiceStatus] failed to cast channel from map")
			return true
		}
		c <- status
		return true
	})
}
