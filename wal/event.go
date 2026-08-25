package wal

import (
	"encoding/json"
)

// HydroEvent is one journal entry: what happened and the handler's own payload.
type HydroEvent struct {
	Type string `json:"type"`
	Item []byte `json:"item"`
}

func NewHydroEvent(typ string, item []byte) *HydroEvent {
	return &HydroEvent{Type: typ, Item: item}
}

func (e HydroEvent) Encode() ([]byte, error) {
	return json.MarshalIndent(e, "", "\t")
}

func decodeHydroEvent(value string) (event HydroEvent, err error) {
	err = json.Unmarshal([]byte(value), &event)
	return event, err
}
