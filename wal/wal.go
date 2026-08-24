package wal

import (
	"context"
)

const eventPrefix = "/events/"

// WAL logs an event before its operation runs and replays the unfinished ones on recovery.
type WAL interface {
	Register(EventHandler)
	Recover(context.Context)
	Log(string, any) (Commit, error)
	Close() error
}

type EventHandler interface {
	Typ() string
	Check(context.Context, any) (need bool, err error)
	Encode(any) ([]byte, error)
	Decode([]byte) (any, error)
	Handle(context.Context, any) error
}

// Commit drops a logged event once its operation has succeeded.
type Commit func() error
