package kv

import (
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
	Put([]byte, []byte) error
	Get([]byte) ([]byte, error)
	Delete([]byte) error
}

// Scanner is the interface that wraps the basic Scan method.
type Scanner interface {
	Scan([]byte) (<-chan ScanEntry, func())
}

// Sequencer is the interface that wraps the basic NextSequence method.
type Sequencer interface {
	NextSequence() (ID uint64, err error)
}

// OpenCloser is the interface that groups the basic Open and Close methods.
type OpenCloser interface {
	Open(path string, mode os.FileMode, timeout time.Duration) error
	Close() error
}

// ScanEntry is the interface that groups the basic Pair and Error methods.
type ScanEntry interface {
	Pair() (key []byte, value []byte)
	Error() error
}
