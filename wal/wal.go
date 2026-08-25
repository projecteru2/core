package wal

import (
	"context"
	"time"

	"github.com/projecteru2/core/lock"
)

// WAL logs an event before its operation runs and replays the unfinished ones on recovery.
type WAL interface {
	Register(EventHandler)
	Recover(context.Context)
	Takeover(ctx context.Context, live []string)
	Log(string, any) (Commit, error)
}

type EventHandler interface {
	Typ() string
	Encode(any) ([]byte, error)
	Decode([]byte) (any, error)
	Handle(context.Context, any) error
}

// Store is the part of the eru store a journal writes through.
type Store interface {
	Put(ctx context.Context, data map[string]string) error
	Delete(ctx context.Context, keys []string) error
	GetPrefix(ctx context.Context, prefix string, limit int64) (map[string]string, error)
	ListPrefix(ctx context.Context, prefix string) ([]string, error)
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)
}

// Commit drops a logged event once its operation has succeeded.
type Commit func() error
