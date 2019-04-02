package calcium

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	sourcemocks "github.com/projecteru2/core/source/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

// DummyLock replace lock for testing
type dummyLock struct {
	m sync.Mutex
}

// Lock for lock
func (d *dummyLock) Lock(ctx context.Context) error {
	d.m.Lock()
	return nil
}

// Unlock for unlock
func (d *dummyLock) Unlock(ctx context.Context) error {
	d.m.Unlock()
	return nil
}

func NewTestCluster() *Calcium {
	c := &Calcium{}
	c.config = types.Config{}
	c.store = &storemocks.Store{}
	c.scheduler = &schedulermocks.Scheduler{}
	c.source = &sourcemocks.Source{}
	return c
}

func TestNewCluster(t *testing.T) {
	_, err := New(types.Config{})
	assert.Error(t, err)
}
