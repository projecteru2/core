package kv

import (
	"os"
	"time"
)

// KV is the key/value store backing the WAL.
type KV interface {
	Open(path string, mode os.FileMode, timeout time.Duration) error
	Close() error
	Put([]byte, []byte) error
	Get([]byte) ([]byte, error)
	Delete([]byte) error
	Scan([]byte) (<-chan ScanEntry, func())
	NextSequence() (ID uint64, err error)
}

type ScanEntry interface {
	Pair() (key, value []byte)
	Error() error
}
