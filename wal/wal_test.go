package wal

import (
	"context"
	"maps"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecover(t *testing.T) {
	var handled bool
	handle := func(any) (err error) {
		handled = true
		return err
	}

	var encoded bool
	encode := func(any) (bs []byte, err error) {
		encoded = true
		return bs, err
	}

	var decoded bool
	decode := func([]byte) (item any, err error) {
		decoded = true
		return item, err
	}

	var wal WAL
	wal, err := NewHydro(context.Background(), newMemStore(), "127.0.0.1:5001", testConfig())
	require.NoError(t, err)
	defer wal.Close()

	eventype := "create"
	wal.Register(simpleEventHandler{
		event:  eventype,
		encode: encode,
		decode: decode,
		handle: handle,
	})

	_, err = wal.Log(eventype, struct{}{})
	require.NoError(t, err)

	wal.Recover(context.Background())
	assert.True(t, handled)
	assert.True(t, encoded)
	assert.True(t, decoded)
}

type simpleEventHandler struct {
	event  string
	encode func(any) ([]byte, error)
	decode func([]byte) (any, error)
	handle func(any) error
}

func (h simpleEventHandler) Typ() string {
	return h.event
}

func (h simpleEventHandler) Encode(raw any) ([]byte, error) {
	return h.encode(raw)
}

func (h simpleEventHandler) Decode(bs []byte) (any, error) {
	return h.decode(bs)
}

func (h simpleEventHandler) Handle(ctx context.Context, raw any) error {
	return h.handle(raw)
}

// memStore is an in-memory Store whose reads and writes can be made to fail.
type memStore struct {
	sync.Mutex

	data   map[string]string
	getErr error
	putErr error
}

func newMemStore() *memStore {
	return &memStore{data: map[string]string{}}
}

func (s *memStore) Put(_ context.Context, data map[string]string) error {
	s.Lock()
	defer s.Unlock()
	if s.putErr != nil {
		return s.putErr
	}
	maps.Copy(s.data, data)
	return nil
}

func (s *memStore) Delete(_ context.Context, keys []string) error {
	s.Lock()
	defer s.Unlock()
	for _, key := range keys {
		delete(s.data, key)
	}
	return nil
}

func (s *memStore) GetPrefix(_ context.Context, prefix string, _ int64) (map[string]string, error) {
	s.Lock()
	defer s.Unlock()
	if s.getErr != nil {
		return nil, s.getErr
	}

	data := map[string]string{}
	for key, value := range s.data {
		if strings.HasPrefix(key, prefix) {
			data[key] = value
		}
	}
	return data, nil
}
