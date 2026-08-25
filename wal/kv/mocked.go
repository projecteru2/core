package kv

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// MockedKV is an in-memory KV for tests.
type MockedKV struct {
	sync.Mutex
	pool    sync.Map
	nextSeq uint64
}

func NewMockedKV() *MockedKV {
	return &MockedKV{nextSeq: 1}
}

func (m *MockedKV) Open(string, os.FileMode, time.Duration) error {
	return nil
}

func (m *MockedKV) Close() error {
	m.pool.Clear()
	return nil
}

func (m *MockedKV) Put(key, value []byte) error {
	m.pool.Store(string(key), value)
	return nil
}

func (m *MockedKV) PutNext(entry SequencedEntry) error {
	m.Lock()
	defer m.Unlock()
	key, value, err := entry(m.nextSeq)
	if err != nil {
		return err
	}
	m.nextSeq++
	m.pool.Store(string(key), value)
	return nil
}

func (m *MockedKV) Get(key []byte) ([]byte, error) {
	value, ok := m.pool.Load(string(key))
	if !ok {
		return nil, errors.Wrapf(types.ErrInvaildCount, "no such key: %s", key)
	}
	return value.([]byte), nil
}

func (m *MockedKV) Delete(key []byte) error {
	m.pool.Delete(string(key))
	return nil
}

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
			m.pool.Range(func(k, v any) bool {
				dataCh <- MockedScanEntry{Key: k.(string), Value: v.([]byte)}
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

// MockedScanEntry is a key/value pair produced by MockedKV.Scan.
type MockedScanEntry struct {
	Err   error
	Key   string
	Value []byte
}

func (e MockedScanEntry) Pair() ([]byte, []byte) {
	return []byte(e.Key), e.Value
}

func (e MockedScanEntry) Error() error {
	return e.Err
}
