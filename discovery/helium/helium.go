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

// Helium .
type Helium struct {
	sync.Once
	config types.GRPCConfig
	stor   store.Store
	subs   hashmap.HashMap
}

// New .
func New(config types.GRPCConfig, stor store.Store) *Helium {
	h := &Helium{config: config, stor: stor, subs: hashmap.HashMap{}}
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
