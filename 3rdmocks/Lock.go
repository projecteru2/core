package mocks

import (
	context "context"
	"sync"
)

// DummyLock replace lock for testing
type DummyLock struct {
	m sync.Mutex
}

// Lock for lock
func (d *DummyLock) Lock(ctx context.Context) error {
	d.m.Lock()
	return nil
}

// Unlock for unlock
func (d *DummyLock) Unlock(ctx context.Context) error {
	d.m.Unlock()
	return nil
}
