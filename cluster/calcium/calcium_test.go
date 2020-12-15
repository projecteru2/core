package calcium

import (
	"context"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	sourcemocks "github.com/projecteru2/core/source/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

// DummyLock replace lock for testing
type dummyLock struct {
	m sync.Mutex
}

// Lock for lock
func (d *dummyLock) Lock(ctx context.Context) (context.Context, error) {
	d.m.Lock()
	return context.Background(), nil
}

// Unlock for unlock
func (d *dummyLock) Unlock(ctx context.Context) error {
	d.m.Unlock()
	return nil
}

func NewTestCluster() *Calcium {
	c := &Calcium{}
	c.config = types.Config{
		GlobalTimeout: 30 * time.Second,
		Git: types.GitConfig{
			CloneTimeout: 300 * time.Second,
		},
		Scheduler: types.SchedConfig{
			MaxShare:  -1,
			ShareBase: 100,
		},
	}
	c.store = &storemocks.Store{}
	c.scheduler = &schedulermocks.Scheduler{}
	c.source = &sourcemocks.Source{}
	return c
}

func TestNewCluster(t *testing.T) {
	_, err := New(types.Config{}, false)
	assert.Error(t, err)
	c, err := New(types.Config{}, true)
	assert.NoError(t, err)
	c.Finalizer()
	privFile, err := ioutil.TempFile("", "priv")
	assert.NoError(t, err)
	_, err = privFile.WriteString("privkey")
	assert.NoError(t, err)
	defer privFile.Close()
	go func() {
		c, err := New(types.Config{Git: types.GitConfig{SCMType: "gitlab", PrivateKey: privFile.Name()}}, true)
		assert.NoError(t, err)
		c.Finalizer()
	}()
	go func() {
		c, err := New(types.Config{Git: types.GitConfig{SCMType: "github", PrivateKey: privFile.Name()}}, true)
		assert.NoError(t, err)
		c.Finalizer()
	}()
}

func TestFinalizer(t *testing.T) {
	c := NewTestCluster()
	store := &storemocks.Store{}
	c.store = store
	store.On("TerminateEmbededStorage").Return(nil)
	c.Finalizer()
}
