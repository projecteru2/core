package helium

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"

	"github.com/cornelk/hashmap"
	"github.com/google/uuid"
)

const interval = 15 * time.Second

// Helium .
type Helium struct {
	sync.Once
	store     store.Store
	subs      *hashmap.Map[uint32, entry]
	interval  time.Duration
	unsubChan chan uint32
}

type entry struct {
	ch     chan types.ServiceStatus
	ctx    context.Context
	cancel context.CancelFunc
}

// New .
func New(config types.GRPCConfig, store store.Store) *Helium {
	h := &Helium{
		interval:  config.ServiceDiscoveryPushInterval,
		store:     store,
		subs:      hashmap.New[uint32, entry](),
		unsubChan: make(chan uint32),
	}
	if h.interval < time.Second {
		h.interval = interval
	}
	h.Do(func() {
		h.start(context.TODO()) // TODO rewrite ctx here, because this will run only once!
	})
	return h
}

// Subscribe .
func (h *Helium) Subscribe(ctx context.Context) (uuid.UUID, <-chan types.ServiceStatus) {
	id := uuid.New()
	key := id.ID()
	subCtx, cancel := context.WithCancel(ctx)
	ch := make(chan types.ServiceStatus)
	h.subs.Set(key, entry{
		ch:     ch,
		ctx:    subCtx,
		cancel: cancel,
	})
	return id, ch
}

// Unsubscribe .
func (h *Helium) Unsubscribe(id uuid.UUID) {
	h.unsubChan <- id.ID()
}

func (h *Helium) start(ctx context.Context) {
	ch, err := h.store.ServiceStatusStream(ctx)
	if err != nil {
		log.Error(ctx, err, "[WatchServiceStatus] failed to start watch")
		return
	}

	go func() {
		log.Info(ctx, "[WatchServiceStatus] service discovery start")
		defer log.Warn(ctx, "[WatchServiceStatus] service discovery exited")
		var latestStatus types.ServiceStatus
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case addresses, ok := <-ch:
				if !ok {
					log.Warn(ctx, "[WatchServiceStatus] watch channel closed")
					return
				}

				latestStatus = types.ServiceStatus{
					Addresses: addresses,
					Interval:  h.interval * 2,
				}

			case id := <-h.unsubChan:
				if entry, ok := h.subs.Get(id); ok {
					entry.cancel()
					h.subs.Del(id)
					close(entry.ch)
				}

			case <-ticker.C:
			}

			h.dispatch(latestStatus)
		}
	}()
}

func (h *Helium) dispatch(status types.ServiceStatus) {
	f := func(key uint32, val entry) {
		defer func() {
			if err := recover(); err != nil {
				log.Errorf(nil, errors.Errorf("%+v", err), "[dispatch] dispatch %+v failed", key) //nolint
			}
		}()
		select {
		case val.ch <- status:
			return
		case <-val.ctx.Done():
			return
		}
	}
	h.subs.Range(func(k uint32, v entry) bool {
		f(k, v)
		return true
	})
}
