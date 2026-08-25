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
	ID   uint64 `json:"ID"`
	Type string `json:"type"`
	Item []byte `json:"item"`
}

func NewHydroEvent(ID uint64, typ string, item []byte) *HydroEvent {
	return &HydroEvent{ID: ID, Type: typ, Item: item}
}

func (e HydroEvent) Encode() ([]byte, error) {
	return json.MarshalIndent(e, "", "\t")
}

func (e HydroEvent) Key() []byte {
	return []byte(filepath.Join(eventPrefix, fmt.Sprintf("%016x", e.ID)))
}

func parseHydroEventID(key []byte) (uint64, error) {
	ID := strings.TrimLeft(strings.TrimPrefix(string(key), eventPrefix), "0")
	return strconv.ParseUint(ID, 16, 64)
}
