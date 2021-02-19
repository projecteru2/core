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
	Log(context.Context, string, interface{}) (Commit, error)
}

// OpenCloser is the interface that groups the basic Open and Close methods.
type OpenCloser interface {
	Open(context.Context, string, time.Duration) error
	Close(context.Context) error
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
func (h SimpleEventHandler) Check(raw interface{}) (bool, error) {
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
func (h SimpleEventHandler) Handle(raw interface{}) error {
	return h.handle(raw)
}

// EventHandler is the interface that groups a few methods.
type EventHandler interface {
	Event() string
	Check(interface{}) (need bool, err error)
	Encode(interface{}) ([]byte, error)
	Decode([]byte) (interface{}, error)
	Handle(interface{}) error
}

// Commit is a function for committing an event log.
type Commit func(context.Context) error

// Register registers a new event to doit.
func Register(handler EventHandler) {
	wal.Register(handler)
}

// Log records a log item.
func Log(ctx context.Context, event string, item interface{}) (Commit, error) {
	return wal.Log(ctx, event, item)
}

// Recover makes a disaster recovery.
func Recover(ctx context.Context) {
	wal.Recover(ctx)
}

// Close closes a WAL file.
func Close(ctx context.Context) error {
	return wal.Close(ctx)
}

// Open opens a WAL file.
func Open(ctx context.Context, path string, timeout time.Duration) error {
	return wal.Open(ctx, path, timeout)
}

var wal WAL = NewHydro()
