package kv

import (
	"os"
	"time"
)

// KV is the key/value store backing the WAL.
type KV interface {
	OpenCloser
	Simpler
	Scanner
	Sequencer
}

type Simpler interface {
	Put([]byte, []byte) error
	Get([]byte) ([]byte, error)
	Delete([]byte) error
}

type Scanner interface {
	Scan([]byte) (<-chan ScanEntry, func())
}

type Sequencer interface {
	NextSequence() (ID uint64, err error)
}

type OpenCloser interface {
	Open(path string, mode os.FileMode, timeout time.Duration) error
	Close() error
}

type ScanEntry interface {
	Pair() (key, value []byte)
	Error() error
}
