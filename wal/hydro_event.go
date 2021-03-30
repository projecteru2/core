package wal

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/projecteru2/core/wal/kv"
)

// HydroEvent indicates a log event.
type HydroEvent struct {
	// A global unique identifier.
	ID uint64 `json:"id"`

	// Registered event type name.
	Type string `json:"type"`

	// The encoded log item.
	Item []byte `json:"item"`

	kv kv.KV
}

// NewHydroEvent initializes a new HydroEvent instance.
func NewHydroEvent(kv kv.KV) (e *HydroEvent) {
	e = &HydroEvent{}
	e.kv = kv
	return
}

// Create persists this event.
func (e *HydroEvent) Create() (err error) {
	if e.ID, err = e.kv.NextSequence(); err != nil {
		return
	}

	var value []byte
	if value, err = json.MarshalIndent(e, "", "\t"); err != nil {
		return err
	}

	return e.kv.Put(e.Key(), value)
}

// Delete removes this event from persistence.
func (e HydroEvent) Delete() error {
	return e.kv.Delete(e.Key())
}

// Key returns this event's key path.
func (e HydroEvent) Key() []byte {
	return []byte(filepath.Join(EventPrefix, fmt.Sprintf("%016x", e.ID)))
}

func parseHydroEventID(key []byte) (uint64, error) {
	// Trims the EventPrefix, then trims the padding 0.
	id := strings.TrimLeft(strings.TrimPrefix(string(key), EventPrefix), "0")
	return strconv.ParseUint(id, 16, 64)
}
