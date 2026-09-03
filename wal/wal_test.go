package wal

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/lock"
	lockmocks "github.com/projecteru2/core/lock/mocks"
)

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

type memStore struct {
	sync.Mutex

	data    map[string]string
	getErr  error
	putErr  error
	lockErr error
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

func (s *memStore) ListPrefix(ctx context.Context, prefix string) ([]string, error) {
	data, err := s.GetPrefix(ctx, prefix, 0)
	if err != nil {
		return nil, err
	}
	return slices.Sorted(maps.Keys(data)), nil
}

func (s *memStore) CreateLock(_ string, _ time.Duration) (lock.DistributedLock, error) {
	if s.lockErr != nil {
		return nil, s.lockErr
	}
	journalLock := &lockmocks.DistributedLock{}
	journalLock.On("Lock", mock.Anything).Return(context.Background(), nil)
	journalLock.On("Unlock", mock.Anything).Return(nil)
	return journalLock, nil
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
