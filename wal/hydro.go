package wal

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/projecteru2/core/log"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal/kv"
)

// Hydro is the simplest wal implementation.
type Hydro struct {
	handlers sync.Map
	kv       kv.KV
}

// NewHydro initailizes a new Hydro instance.
func NewHydro() *Hydro {
	return &Hydro{
		kv: kv.NewLithium(),
	}
}

// Open connects a kvdb.
func (h *Hydro) Open(path string, timeout time.Duration) (err error) {
	err = h.kv.Open(path, 0600, timeout)
	return
}

// Close disconnects the kvdb.
func (h *Hydro) Close() error {
	return h.kv.Close()
}

// Register registers a new event handler.
func (h *Hydro) Register(handler EventHandler) {
	h.handlers.Store(handler.Event(), handler)
}

// Recover starts a disaster recovery, which will replay all the events.
func (h *Hydro) Recover(ctx context.Context) {
	ch, _ := h.kv.Scan([]byte(EventPrefix))

	events := []HydroEvent{}
	for ent := range ch {
		ev, err := h.decodeEvent(ent)
		if err != nil {
			log.Errorf(nil, "[Recover] decode event error: %v", err) //nolint
			continue
		}
		events = append(events, ev)
	}

	for _, ev := range events {
		handler, ok := h.getEventHandler(ev.Type)
		if !ok {
			log.Errorf(nil, "[Recover] no such event handler for %s", ev.Type) //nolint
			continue
		}

		if err := h.recover(ctx, handler, ev); err != nil {
			log.Errorf(nil, "[Recover] handle event %d (%s) failed: %v", ev.ID, ev.Type, err) //nolint
			continue
		}
	}
}

// Log records a log item.
func (h *Hydro) Log(eventype string, item interface{}) (Commit, error) {
	handler, ok := h.getEventHandler(eventype)
	if !ok {
		return nil, coretypes.NewDetailedErr(coretypes.ErrUnregisteredWALEventType, eventype)
	}

	bs, err := handler.Encode(item)
	if err != nil {
		return nil, err
	}

	event := NewHydroEvent(h.kv)
	event.Type = eventype
	event.Item = bs

	if err = event.Create(); err != nil {
		return nil, err
	}

	return event.Delete, nil
}

func (h *Hydro) recover(ctx context.Context, handler EventHandler, event HydroEvent) error {
	item, err := handler.Decode(event.Item)
	if err != nil {
		return err
	}

	switch handle, err := handler.Check(ctx, item); {
	case err != nil:
		return err
	case !handle:
		return event.Delete()
	}

	if err := handler.Handle(ctx, item); err != nil {
		return err
	}

	return event.Delete()
}

func (h *Hydro) getEventHandler(event string) (handler EventHandler, ok bool) {
	var raw interface{}
	if raw, ok = h.handlers.Load(event); !ok {
		return
	}

	handler, ok = raw.(EventHandler)

	return
}

func (h *Hydro) decodeEvent(ent kv.ScanEntry) (event HydroEvent, err error) {
	if err = ent.Error(); err != nil {
		return
	}

	key, value := ent.Pair()
	if err = json.Unmarshal(value, &event); err != nil {
		return
	}

	event.kv = h.kv
	event.ID, err = parseHydroEventID(key)

	return
}
