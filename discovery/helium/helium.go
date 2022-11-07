package helium

import (
	"context"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
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
func New(ctx context.Context, config types.GRPCConfig, store store.Store) *Helium {
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
		h.start(ctx)
	})
	return h
}

// Subscribe .
func (h *Helium) Subscribe(ctx context.Context) (uuid.UUID, <-chan types.ServiceStatus) {
	ID := uuid.New()
	key := ID.ID()
	subCtx, cancel := context.WithCancel(ctx)
	ch := make(chan types.ServiceStatus)
	h.subs.Set(key, entry{
		ch:     ch,
		ctx:    subCtx,
		cancel: cancel,
	})
	return ID, ch
}

// Unsubscribe .
func (h *Helium) Unsubscribe(ID uuid.UUID) {
	h.unsubChan <- ID.ID()
}

func (h *Helium) start(ctx context.Context) {
	logger := log.WithFunc("helium.start")
	ch, err := h.store.ServiceStatusStream(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to start watch")
		return
	}

	go func() {
		logger.Info(ctx, "service discovery start")
		defer logger.Warn(ctx, "service discovery exited")
		var latestStatus types.ServiceStatus
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case addresses, ok := <-ch:
				if !ok {
					logger.Warn(ctx, "watch channel closed")
					return
				}

				latestStatus = types.ServiceStatus{
					Addresses: addresses,
					Interval:  h.interval * 2,
				}

			case ID := <-h.unsubChan:
				if entry, ok := h.subs.Get(ID); ok {
					entry.cancel()
					h.subs.Del(ID)
					close(entry.ch)
				}

			case <-ticker.C:
			}

			h.dispatch(ctx, latestStatus)
		}
	}()
}

func (h *Helium) dispatch(ctx context.Context, status types.ServiceStatus) {
	f := func(key uint32, val entry) {
		defer func() {
			if err := recover(); err != nil {
				log.WithFunc("helium.dispatch").Errorf(ctx, errors.Errorf("%+v", err), "dispatch %+v failed", key)
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
