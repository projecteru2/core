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

const (
	fileMode = 0600
)

// Hydro is the simplest wal implementation.
type Hydro struct {
	*haxmap.Map[string, EventHandler]
	store kv.KV
}

// NewHydro initailizes a new Hydro instance.
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

// Close disconnects the kvdb.
func (h *Hydro) Close() error {
	return h.store.Close()
}

// Register registers a new event handler.
func (h *Hydro) Register(handler EventHandler) {
	h.Map.Set(handler.Typ(), handler)
}

// Recover starts a disaster recovery, which will replay all the events.
func (h *Hydro) Recover(ctx context.Context) {
	ch, _ := h.store.Scan([]byte(eventPrefix))
	events := []HydroEvent{}
	logger := log.WithFunc("wal.hydro.Recover")

	for {
		scanEntry, ok := <-ch
		if !ok {
			logger.Warn(ctx, "noting have to restore, wal recover closed")
			break
		}

		event, err := h.decodeEvent(scanEntry)
		if err != nil {
			logger.Error(ctx, err, "decode event error")
			continue
		}
		events = append(events, event)
	}

	for _, event := range events {
		handler, ok := h.getEventHandler(event.Type)
		if !ok {
			logger.Warn(ctx, "no such event handler for %s", event.Type)
			continue
		}

		if err := h.recover(ctx, handler, event); err != nil {
			logger.Errorf(ctx, err, "handle event %d (%s) failed", event.ID, event.Type)
			continue
		}
	}
}

// Log records a log item.
func (h *Hydro) Log(eventyp string, item interface{}) (Commit, error) {
	handler, ok := h.getEventHandler(eventyp)
	if !ok {
		return nil, errors.Wrap(coretypes.ErrInvaildWALEventType, eventyp)
	}

	bs, err := handler.Encode(item) // TODO 2 times encode is necessary?
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

	del := func() error {
		return h.store.Delete(event.Key())
	}

	switch handle, err := handler.Check(ctx, item); {
	case err != nil:
		return err
	case !handle:
		return del()
	default:
		if err := handler.Handle(ctx, item); err != nil {
			return err
		}
	}
	return del()
}

func (h *Hydro) getEventHandler(eventyp string) (EventHandler, bool) {
	handler, ok := h.Map.Get(eventyp)
	if !ok {
		return nil, ok
	}
	return handler, ok
}

func (h *Hydro) decodeEvent(scanEntry kv.ScanEntry) (event HydroEvent, err error) {
	if err = scanEntry.Error(); err != nil {
		return
	}

	key, value := scanEntry.Pair()
	if err = json.Unmarshal(value, &event); err != nil {
		return
	}

	event.ID, err = parseHydroEventID(key)
	return
}
