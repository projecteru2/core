package wal

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// HydroEvent indicates a log event.
type HydroEvent struct {
	// A global unique identifier.
	ID uint64 `json:"id"`

	// Registered event type name.
	Type string `json:"type"`

	// The encoded log item.
	Item []byte `json:"item"`
}

// NewHydroEvent initializes a new HydroEvent instance.
func NewHydroEvent(ID uint64, typ string, item []byte) *HydroEvent {
	return &HydroEvent{ID: ID, Type: typ, Item: item}
}

// Encode this event
func (e HydroEvent) Encode() ([]byte, error) {
	return json.MarshalIndent(e, "", "\t")
}

// Key returns this event's key path.
func (e HydroEvent) Key() []byte {
	return []byte(filepath.Join(eventPrefix, fmt.Sprintf("%016x", e.ID)))
}

func parseHydroEventID(key []byte) (uint64, error) {
	// Trims the EventPrefix, then trims the padding 0.
	id := strings.TrimLeft(strings.TrimPrefix(string(key), eventPrefix), "0")
	return strconv.ParseUint(id, 16, 64)
}
