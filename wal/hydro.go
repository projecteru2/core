package wal

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	eventKey    = "/wal/%s/%016x"
	eventPrefix = "/wal/%s/"
)

// Hydro journals events into the eru store, under the prefix of this instance's service address.
type Hydro struct {
	handlers sync.Map
	seq      atomic.Uint64

	store   Store
	ctx     context.Context
	config  coretypes.Config
	address string
}

func NewHydro(ctx context.Context, store Store, address string, config coretypes.Config) (*Hydro, error) {
	// the journal outlives every request that writes to it, so it keeps a context of its own
	hydro := &Hydro{store: store, ctx: utils.NewInheritCtx(ctx), config: config, address: address}
	seq, err := hydro.lastSeq(ctx)
	if err != nil {
		return nil, err
	}
	hydro.seq.Store(seq)
	return hydro, nil
}

func (h *Hydro) Close() error {
	return nil
}

func (h *Hydro) Register(handler EventHandler) {
	h.handlers.Store(handler.Typ(), handler)
}

func (h *Hydro) Recover(ctx context.Context) {
	h.recoverAddress(ctx, h.address)
}

func (h *Hydro) Log(eventyp string, item any) (Commit, error) {
	handler, ok := h.handler(eventyp)
	if !ok {
		return nil, errors.Wrap(coretypes.ErrInvaildWALEventType, eventyp)
	}

	bs, err := handler.Encode(item)
	if err != nil {
		return nil, err
	}
	value, err := NewHydroEvent(eventyp, bs).Encode()
	if err != nil {
		return nil, coretypes.ErrInvaildWALEvent
	}

	key := fmt.Sprintf(eventKey, h.address, h.seq.Add(1))
	ctx, cancel := h.storeContext()
	defer cancel()
	if err = h.store.Put(ctx, map[string]string{key: string(value)}); err != nil {
		return nil, err
	}

	return func() error {
		ctx, cancel := h.storeContext()
		defer cancel()
		return h.store.Delete(ctx, []string{key})
	}, nil
}

func (h *Hydro) recoverAddress(ctx context.Context, address string) {
	logger := log.WithFunc("wal.hydro.recoverAddress").WithField("address", address)
	events, err := h.store.GetPrefix(ctx, fmt.Sprintf(eventPrefix, address), 0)
	if err != nil {
		logger.Error(ctx, err, "read journal")
		return
	}

	for _, key := range slices.Sorted(maps.Keys(events)) {
		event, err := decodeHydroEvent(events[key])
		if err != nil {
			logger.Errorf(ctx, err, "decode event %s", key)
			continue
		}

		handler, ok := h.handler(event.Type)
		if !ok {
			logger.Warnf(ctx, "no such event handler for %s", event.Type)
			continue
		}

		if err := h.handle(ctx, handler, event, key); err != nil {
			logger.Errorf(ctx, err, "handle event %s (%s) failed", key, event.Type)
		}
	}
}

func (h *Hydro) handle(ctx context.Context, handler EventHandler, event HydroEvent, key string) error {
	item, err := handler.Decode(event.Item)
	if err != nil {
		return err
	}

	if err := handler.Handle(ctx, item); err != nil {
		return err
	}
	return h.store.Delete(ctx, []string{key})
}

func (h *Hydro) handler(eventyp string) (EventHandler, bool) {
	v, ok := h.handlers.Load(eventyp)
	if !ok {
		return nil, false
	}
	return v.(EventHandler), true
}

func (h *Hydro) lastSeq(ctx context.Context) (uint64, error) {
	events, err := h.store.GetPrefix(ctx, fmt.Sprintf(eventPrefix, h.address), 0)
	if err != nil {
		return 0, err
	}

	var last uint64
	for key := range events {
		seq, err := strconv.ParseUint(utils.Tail(key), 16, 64)
		if err != nil {
			continue
		}
		last = max(last, seq)
	}
	return last, nil
}

func (h *Hydro) storeContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(h.ctx, h.config.GlobalTimeout)
}
