package wal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal/kv"
)

const fileMode = 0o600

// Hydro is the simplest wal implementation.
type Hydro struct {
	*haxmap.Map[string, EventHandler]
	store kv.KV
}

func NewHydro(path string, timeout time.Duration) (*Hydro, error) {
	store := kv.NewLithium()
	if err := store.Open(path, fileMode, timeout); err != nil {
		return nil, err
	}
	return &Hydro{
		Map:   haxmap.New[string, EventHandler](),
		store: store,
	}, nil
}

func (h *Hydro) Close() error {
	return h.store.Close()
}

func (h *Hydro) Register(handler EventHandler) {
	h.Set(handler.Typ(), handler)
}

func (h *Hydro) Recover(ctx context.Context) {
	ch, _ := h.store.Scan([]byte(eventPrefix))
	logger := log.WithFunc("wal.hydro.Recover")

	var events []HydroEvent
	for scanEntry := range ch {
		event, err := h.decodeEvent(scanEntry)
		if err != nil {
			logger.Error(ctx, err, "decode event")
			continue
		}
		events = append(events, event)
	}

	for _, event := range events {
		handler, ok := h.Get(event.Type)
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
	handler, ok := h.Get(eventyp)
	if !ok {
		return nil, errors.Wrap(coretypes.ErrInvaildWALEventType, eventyp)
	}

	bs, err := handler.Encode(item)
	if err != nil {
		return nil, err
	}

	var ID uint64
	if ID, err = h.store.NextSequence(); err != nil {
		return nil, err
	}

	event := NewHydroEvent(ID, eventyp, bs)
	if bs, err = event.Encode(); err != nil {
		return nil, coretypes.ErrInvaildWALEvent
	}

	if err = h.store.Put(event.Key(), bs); err != nil {
		return nil, err
	}

	return func() error {
		return h.store.Delete(event.Key())
	}, nil
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
	if err = scanEntry.Error(); err != nil {
		return event, err
	}

	key, value := scanEntry.Pair()
	if err = json.Unmarshal(value, &event); err != nil {
		return event, err
	}

	event.ID, err = parseHydroEventID(key)
	return event, err
}
