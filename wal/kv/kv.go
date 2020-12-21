package kv

import (
	"context"
	"os"
	"time"
)

// KV is the interface that groups the Simpler and Scanner interfaces.
type KV interface {
	OpenCloser
	Simpler
	Scanner
	Sequencer
}

// Simpler is the interface that groups the basic Put, Get and Delete methods.
type Simpler interface {
	Put(context.Context, []byte, []byte) error
	Get(context.Context, []byte) ([]byte, error)
	Delete(context.Context, []byte) error
}

// Scanner is the interface that wraps the basic Scan method.
type Scanner interface {
	Scan(context.Context, []byte) (<-chan ScanEntry, func())
}

// Sequencer is the interface that wraps the basic NextSequence method.
type Sequencer interface {
	NextSequence(context.Context) (id uint64, err error)
}

// OpenCloser is the interface that groups the basic Open and Close methods.
type OpenCloser interface {
	Open(ctx context.Context, path string, mode os.FileMode, timeout time.Duration) error
	Close(context.Context) error
}

// ScanEntry is the interface that groups the basic Pair and Error methods.
type ScanEntry interface {
	Pair() (key []byte, value []byte)
	Error() error
}
