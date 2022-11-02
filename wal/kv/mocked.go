package kv

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/cornelk/hashmap"
	"github.com/projecteru2/core/types"
)

// MockedKV .
type MockedKV struct {
	sync.Mutex
	pool    *hashmap.Map[string, []byte]
	nextSeq uint64
}

// NewMockedKV .
func NewMockedKV() *MockedKV {
	return &MockedKV{
		nextSeq: 1,
		pool:    hashmap.New[string, []byte](),
	}
}

// Open .
func (m *MockedKV) Open(path string, mode os.FileMode, timeout time.Duration) error {
	return nil
}

// Close .
func (m *MockedKV) Close() error {
	m.pool.Range(func(k string, _ []byte) bool {
		m.pool.Del(k)
		return true
	})
	return nil
}

// NextSequence .
func (m *MockedKV) NextSequence() (nextSeq uint64, err error) {
	m.Lock()
	defer m.Unlock()
	nextSeq = m.nextSeq
	m.nextSeq++
	return
}

// Put .
func (m *MockedKV) Put(key, value []byte) (err error) {
	m.pool.Set(string(key), value)
	return
}

// Get .
func (m *MockedKV) Get(key []byte) (value []byte, err error) {
	value, ok := m.pool.Get(string(key))
	if !ok {
		return value, errors.Wrapf(types.ErrInvaildCount, "no such key: %s", key)
	}
	return
}

// Delete .
func (m *MockedKV) Delete(key []byte) (err error) {
	m.pool.Del(string(key))
	return
}

// Scan .
func (m *MockedKV) Scan(prefix []byte) (<-chan ScanEntry, func()) {
	ch := make(chan ScanEntry)

	exit := make(chan struct{})
	abort := func() {
		close(exit)
	}

	go func() {
		defer close(ch)

		dataCh := make(chan MockedScanEntry)
		go func() {
			defer close(dataCh)
			m.pool.Range(func(k string, v []byte) bool {
				dataCh <- MockedScanEntry{Key: k, Value: v}
				return true
			})
		}()

		for {
			select {
			case <-exit:
				return
			case entry, ok := <-dataCh:
				switch {
				case !ok:
					return
				case strings.HasPrefix(entry.Key, string(prefix)):
					ch <- entry
				}
			}
		}
	}()

	return ch, abort
}

// MockedScanEntry .
type MockedScanEntry struct {
	Err   error
	Key   string
	Value []byte
}

// Pair .
func (e MockedScanEntry) Pair() ([]byte, []byte) {
	return []byte(e.Key), e.Value
}

// Error .
func (e MockedScanEntry) Error() error {
	return e.Err
}
