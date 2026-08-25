package wal

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal/kv"
)

const fileMode = 0o600

// Hydro is the simplest wal implementation.
type Hydro struct {
	handlers sync.Map
	store    kv.KV
}

func NewHydro(path string, timeout time.Duration) (*Hydro, error) {
	store := kv.NewLithium()
	if err := store.Open(path, fileMode, timeout); err != nil {
		return nil, err
	}
	return &Hydro{store: store}, nil
}

func (h *Hydro) Close() error {
	return h.store.Close()
}

func (h *Hydro) Register(handler EventHandler) {
	h.handlers.Store(handler.Typ(), handler)
}

func (h *Hydro) Recover(ctx context.Context) {
	logger := log.WithFunc("wal.hydro.Recover")
	ch, abort := h.store.Scan([]byte(eventPrefix))
	defer abort()

	var events []HydroEvent
	for scanEntry := range ch {
		if err := scanEntry.Error(); err != nil {
			logger.Error(ctx, err, "scan events")
			return
		}
		event, err := h.decodeEvent(scanEntry)
		if err != nil {
			logger.Error(ctx, err, "decode event")
			continue
		}
		events = append(events, event)
	}

	for _, event := range events {
		handler, ok := h.handler(event.Type)
		if !ok {
			logger.Warnf(ctx, "no such event handler for %s", event.Type)
			continue
		}

		if err := h.recover(ctx, handler, event); err != nil {
			logger.Errorf(ctx, err, "handle event %d (%s) failed", event.ID, event.Type)
			continue
		}
	}
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

	var key []byte
	if err = h.store.PutNext(func(seq uint64) ([]byte, []byte, error) {
		event := NewHydroEvent(seq, eventyp, bs)
		value, encodeErr := event.Encode()
		if encodeErr != nil {
			return nil, nil, coretypes.ErrInvaildWALEvent
		}
		key = event.Key()
		return key, value, nil
	}); err != nil {
		return nil, err
	}

	return func() error {
		return h.store.Delete(key)
	}, nil
}

func (h *Hydro) handler(eventyp string) (EventHandler, bool) {
	v, ok := h.handlers.Load(eventyp)
	if !ok {
		return nil, false
	}
	return v.(EventHandler), true
}

func (h *Hydro) recover(ctx context.Context, handler EventHandler, event HydroEvent) error {
	item, err := handler.Decode(event.Item)
	if err != nil {
		return err
	}

	if err := handler.Handle(ctx, item); err != nil {
		return err
	}
	return h.store.Delete(event.Key())
}

func (h *Hydro) decodeEvent(scanEntry kv.ScanEntry) (event HydroEvent, err error) {
	key, value := scanEntry.Pair()
	if err = json.Unmarshal(value, &event); err != nil {
		return event, err
	}

	event.ID, err = parseHydroEventID(key)
	return event, err
}
