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
	PutNext(SequencedEntry) error
	Get([]byte) ([]byte, error)
	Delete([]byte) error
	Scan([]byte) (<-chan ScanEntry, func())
}

type SequencedEntry func(seq uint64) (key, value []byte, err error)

type ScanEntry interface {
	Pair() (key, value []byte)
	Error() error
}
