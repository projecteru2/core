package kv

import (
	"os"
	"strings"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// MockedKV is an in-memory KV for tests.
type MockedKV struct {
	sync.Mutex
	pool    *haxmap.Map[string, []byte]
	nextSeq uint64
}

func NewMockedKV() *MockedKV {
	return &MockedKV{
		nextSeq: 1,
		pool:    haxmap.New[string, []byte](),
	}
}

func (m *MockedKV) Open(string, os.FileMode, time.Duration) error {
	return nil
}

func (m *MockedKV) Close() error {
	m.pool.ForEach(func(k string, _ []byte) bool {
		m.pool.Del(k)
		return true
	})
	return nil
}

func (m *MockedKV) NextSequence() (nextSeq uint64, err error) {
	m.Lock()
	defer m.Unlock()
	nextSeq = m.nextSeq
	m.nextSeq++
	return nextSeq, err
}

func (m *MockedKV) Put(key, value []byte) (err error) {
	m.pool.Set(string(key), value)
	return err
}

func (m *MockedKV) Get(key []byte) (value []byte, err error) {
	value, ok := m.pool.Get(string(key))
	if !ok {
		return value, errors.Wrapf(types.ErrInvaildCount, "no such key: %s", key)
	}
	return value, err
}

func (m *MockedKV) Delete(key []byte) (err error) {
	m.pool.Del(string(key))
	return err
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
			m.pool.ForEach(func(k string, v []byte) bool {
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
