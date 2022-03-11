package wal

import (
	"context"
)

const (
	eventPrefix = "/events/"
)

// WAL is the interface that groups the Register and Recover interfaces.
type WAL interface {
	Registry
	Recoverer
	Logger
	Closer
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

// Closer is the interface that groups the Close methods.
type Closer interface {
	Close() error
}

// EventHandler is the interface that groups a few methods.
type EventHandler interface {
	Typ() string
	Check(context.Context, interface{}) (need bool, err error)
	Encode(interface{}) ([]byte, error)
	Decode([]byte) (interface{}, error)
	Handle(context.Context, interface{}) error
}

// Commit is a function for committing an event log.
type Commit func() error
