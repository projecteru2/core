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

// EventHandler indicates a handler of a specific event.
type EventHandler struct {
	Event  string
	Check  Check
	Encode Encode
	Decode Decode
	Handle Handle
}

// Encode is a function to encode a log item
type Encode func(interface{}) ([]byte, error)

// Decode is a function to decode bytes to an interface{}
type Decode func([]byte) (interface{}, error)

// Handle is a function to play a log item.
type Handle func(interface{}) error

// Check is a function for checking a log item whether need to be played it.
type Check func(interface{}) (need bool, err error)

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
