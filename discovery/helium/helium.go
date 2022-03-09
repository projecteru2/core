package helium

import (
	"context"
	"sync"
	"time"

	"github.com/cornelk/hashmap"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"

	"github.com/google/uuid"
)

const interval = 15 * time.Second

// Helium .
type Helium struct {
	sync.Once
	stor     store.Store
	subs     hashmap.HashMap
	interval time.Duration
}

// New .
func New(config types.GRPCConfig, stor store.Store) *Helium {
	h := &Helium{interval: config.ServiceDiscoveryPushInterval, stor: stor, subs: hashmap.HashMap{}}
	if h.interval < time.Second {
		h.interval = interval
	}
	h.Do(func() {
		h.start(context.TODO()) // TODO rewrite ctx here, because this will run only once!
	})
	return h
}

// Subscribe .
func (h *Helium) Subscribe(ch chan<- types.ServiceStatus) uuid.UUID {
	id := uuid.New()
	key := id.ID()
	h.subs.Set(key, ch)
	return id
}

// Unsubscribe .
func (h *Helium) Unsubscribe(id uuid.UUID) {
	v, ok := h.subs.GetUintKey(uintptr(id.ID()))
	if !ok {
		return
	}
	ch, ok := v.(chan<- types.ServiceStatus)
	if !ok {
		return
	}
	close(ch)
	h.subs.Del(id.ID())
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
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case addresses, ok := <-ch:
				if !ok {
					log.Error("[WatchServiceStatus] watch channel closed")
					return
				}

				latestStatus = types.ServiceStatus{
					Addresses: addresses,
					Interval:  h.interval * 2,
				}
			case <-ticker.C:
			}
			h.dispatch(latestStatus)
		}
	}()
}

func (h *Helium) dispatch(status types.ServiceStatus) {
	f := func(kv hashmap.KeyValue) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf(context.TODO(), "[dispatch] dispatch %v failed, err: %v", kv.Key, err)
			}
		}()
		ch, ok := kv.Value.(chan<- types.ServiceStatus)
		if !ok {
			log.Error("[WatchServiceStatus] failed to cast channel from map")
		}
		ch <- status
	}
	for kv := range h.subs.Iter() {
		f(kv)
	}
}
