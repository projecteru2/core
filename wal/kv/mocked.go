package kv

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cornelk/hashmap"
)

// MockedKV .
type MockedKV struct {
	sync.Mutex
	pool    hashmap.HashMap
	nextSeq uint64
}

// NewMockedKV .
func NewMockedKV() *MockedKV {
	return &MockedKV{
		nextSeq: 1,
	}
}

// Open .
func (m *MockedKV) Open(path string, mode os.FileMode, timeout time.Duration) error {
	return nil
}

// Close .
func (m *MockedKV) Close() error {
	for kv := range m.pool.Iter() {
		m.pool.Del(kv.Key)
	}
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
	raw, ok := m.pool.GetStringKey(string(key))
	if !ok {
		err = fmt.Errorf("no such key: %s", key)
		return
	}

	if value, ok = raw.([]byte); !ok {
		err = fmt.Errorf("value must be a []byte, but %v", raw)
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
		dataCh := m.pool.Iter()

		for {
			select {
			case <-exit:
				return
			case kv := <-dataCh:
				var entry MockedScanEntry
				var ok bool

				if kv.Key == nil {
					return
				}

				if entry.Key, ok = kv.Key.(string); !ok {
					entry.Err = fmt.Errorf("key must be a string, but %v", kv.Key)
					continue
				}

				if !strings.HasPrefix(entry.Key, string(prefix)) {
					continue
				}

				if entry.Value, ok = kv.Value.([]byte); !ok {
					entry.Err = fmt.Errorf("value must be a []byte, but %v", kv.Value)
					continue
				}
				ch <- entry
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
