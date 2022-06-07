package wal

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cornelk/hashmap"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal/kv"
)

const (
	fileMode = 0600
)

// Hydro is the simplest wal implementation.
type Hydro struct {
	hashmap.HashMap
	stor kv.KV
}

// NewHydro initailizes a new Hydro instance.
func NewHydro(path string, timeout time.Duration) (*Hydro, error) {
	stor := kv.NewLithium()
	if err := stor.Open(path, fileMode, timeout); err != nil {
		return nil, err
	}
	return &Hydro{HashMap: hashmap.HashMap{}, stor: stor}, nil
}

// Close disconnects the kvdb.
func (h *Hydro) Close() error {
	return h.stor.Close()
}

// Register registers a new event handler.
func (h *Hydro) Register(handler EventHandler) {
	h.Set(handler.Typ(), handler)
}

// Recover starts a disaster recovery, which will replay all the events.
func (h *Hydro) Recover(ctx context.Context) {
	ch, _ := h.stor.Scan([]byte(eventPrefix))

	events := []HydroEvent{}
	for scanEntry := range ch {
		event, err := h.decodeEvent(scanEntry)
		if err != nil {
			log.Errorf(nil, "[Recover] decode event error: %v", err) // nolint
			continue
		}
		events = append(events, event)
	}

	for _, event := range events {
		handler, ok := h.getEventHandler(event.Type)
		if !ok {
			log.Errorf(nil, "[Recover] no such event handler for %s", event.Type) // nolint
			continue
		}

		if err := h.recover(ctx, handler, event); err != nil {
			log.Errorf(nil, "[Recover] handle event %d (%s) failed: %v", event.ID, event.Type, err) // nolint
			continue
		}
	}
}

// Log records a log item.
func (h *Hydro) Log(eventyp string, item interface{}) (Commit, error) {
	handler, ok := h.getEventHandler(eventyp)
	if !ok {
		return nil, coretypes.NewDetailedErr(coretypes.ErrUnregisteredWALEventType, eventyp)
	}

	bs, err := handler.Encode(item) // TODO 2 times encode is necessary?
	if err != nil {
		return nil, err
	}

	var id uint64
	if id, err = h.stor.NextSequence(); err != nil {
		return nil, err
	}

	event := NewHydroEvent(id, eventyp, bs)
	if bs, err = event.Encode(); err != nil {
		return nil, coretypes.ErrBadWALEvent
	}

	if err = h.stor.Put(event.Key(), bs); err != nil {
		return nil, err
	}

	return func() error {
		return h.stor.Delete(event.Key())
	}, nil
}

func (h *Hydro) recover(ctx context.Context, handler EventHandler, event HydroEvent) error {
	item, err := handler.Decode(event.Item)
	if err != nil {
		return err
	}

	delete := func() error {
		return h.stor.Delete(event.Key())
	}

	switch handle, err := handler.Check(ctx, item); {
	case err != nil:
		return err
	case !handle:
		return delete()
	default:
		if err := handler.Handle(ctx, item); err != nil {
			return err
		}
	}
	return delete()
}

func (h *Hydro) getEventHandler(eventyp string) (EventHandler, bool) {
	v, ok := h.GetStringKey(eventyp)
	if !ok {
		return nil, ok
	}
	handler, ok := v.(EventHandler)
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
