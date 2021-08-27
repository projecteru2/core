package wal

import (
	"context"
	"time"
)

const (
	// EventPrefix indicates the key prefix of all events' keys.
	EventPrefix = "/events/"
)

// WAL is the interface that groups the Register and Recover interfaces.
type WAL interface {
	Registry
	Recoverer
	Logger
	OpenCloser
}

// Recoverer is the interface that wraps the basic Recover method.
type Recoverer interface {
	Recover(context.Context)
}

// Registry is the interface that wraps the basic Register method.
type Registry interface {
	Register(EventHandler)
}

// Logger is the interface that wraps the basic Log method.
type Logger interface {
	Log(string, interface{}) (Commit, error)
}

// OpenCloser is the interface that groups the basic Open and Close methods.
type OpenCloser interface {
	Open(string, time.Duration) error
	Close() error
}

// SimpleEventHandler simply implements the EventHandler.
type SimpleEventHandler struct {
	event  string
	check  func(raw interface{}) (bool, error)
	encode func(interface{}) ([]byte, error)
	decode func([]byte) (interface{}, error)
	handle func(interface{}) error
}

// Event .
func (h SimpleEventHandler) Event() string {
	return h.event
}

// Check .
func (h SimpleEventHandler) Check(ctx context.Context, raw interface{}) (bool, error) {
	return h.check(raw)
}

// Encode .
func (h SimpleEventHandler) Encode(raw interface{}) ([]byte, error) {
	return h.encode(raw)
}

// Decode .
func (h SimpleEventHandler) Decode(bs []byte) (interface{}, error) {
	return h.decode(bs)
}

// Handle .
func (h SimpleEventHandler) Handle(ctx context.Context, raw interface{}) error {
	return h.handle(raw)
}

// EventHandler is the interface that groups a few methods.
type EventHandler interface {
	Event() string
	Check(context.Context, interface{}) (need bool, err error)
	Encode(interface{}) ([]byte, error)
	Decode([]byte) (interface{}, error)
	Handle(context.Context, interface{}) error
}

// Commit is a function for committing an event log.
type Commit func() error
