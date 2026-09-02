package helium

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
)

const interval = 15 * time.Second

type Helium struct {
	store     store.Store
	subs      sync.Map
	interval  time.Duration
	unsubChan chan uint32
	done      chan struct{}
}

func New(ctx context.Context, config types.GRPCConfig, store store.Store) *Helium {
	h := &Helium{
		interval:  config.ServiceDiscoveryPushInterval,
		store:     store,
		unsubChan: make(chan uint32),
		done:      make(chan struct{}),
	}
	if h.interval < time.Second {
		h.interval = interval
	}
	h.start(ctx)
	return h
}

func (h *Helium) Subscribe() (uint32, <-chan types.ServiceStatus) {
	ch := make(chan types.ServiceStatus, 1)
	for {
		ID := rand.Uint32() //nolint:gosec
		if _, taken := h.subs.LoadOrStore(ID, ch); !taken {
			return ID, ch
		}
	}
}

func (h *Helium) Unsubscribe(ID uint32) {
	select {
	case h.unsubChan <- ID:
	case <-h.done:
	}
}

func (h *Helium) start(ctx context.Context) {
	logger := log.WithFunc("helium.start")
	ch, err := h.store.ServiceStatusStream(ctx)
	if err != nil {
		logger.Error(ctx, err, "failed to start watch")
		close(h.done)
		return
	}

	go func() {
		logger.Info(ctx, "service discovery start")
		defer close(h.done)
		defer logger.Warn(ctx, "service discovery exited")
		var latestStatus types.ServiceStatus
		ticker := time.NewTicker(h.interval)
		defer ticker.Stop()
		for {
			select {
			case addresses, ok := <-ch:
				if !ok {
					logger.Error(ctx, types.ErrMessageChanClosed, "watch channel closed, service discovery is down")
					return
				}

				latestStatus = types.ServiceStatus{
					Addresses: addresses,
					Interval:  h.interval * 2,
				}

			case ID := <-h.unsubChan:
				if v, ok := h.subs.LoadAndDelete(ID); ok {
					close(v.(chan types.ServiceStatus))
				}

			case <-ticker.C:
			}

			h.dispatch(latestStatus)
		}
	}()
}

func (h *Helium) dispatch(status types.ServiceStatus) {
	h.subs.Range(func(_, v any) bool {
		ch := v.(chan types.ServiceStatus)
		select {
		case <-ch:
		default:
		}
		select {
		case ch <- status:
		default:
		}
		return true
	})
}
