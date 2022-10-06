package calcium

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	resourcemocks "github.com/projecteru2/core/resources/mocks"
	sourcemocks "github.com/projecteru2/core/source/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func NewTestCluster() *Calcium {
	walDir, err := os.MkdirTemp(os.TempDir(), "core.wal.*")
	if err != nil {
		panic(err)
	}

	pool, _ := utils.NewPool(20)
	c := &Calcium{pool: pool}
	c.config = types.Config{
		GlobalTimeout: 30 * time.Second,
		Git: types.GitConfig{
			CloneTimeout: 300 * time.Second,
		},
		Scheduler: types.SchedulerConfig{
			MaxShare:  -1,
			ShareBase: 100,
		},
		GRPCConfig: types.GRPCConfig{
			ServiceDiscoveryPushInterval: 15 * time.Second,
		},
		WALFile:             filepath.Join(walDir, "core.wal.log"),
		MaxConcurrency:      100000,
		HAKeepaliveInterval: 16 * time.Second,
	}
	c.store = &storemocks.Store{}
	c.source = &sourcemocks.Source{}
	c.wal = &walmocks.WAL{}
	c.rmgr = &resourcemocks.Manager{}

	mwal := c.wal.(*walmocks.WAL)
	commit := wal.Commit(func() error { return nil })
	mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)
	return c
}

func TestNewCluster(t *testing.T) {
	ctx := context.TODO()
	config := types.Config{WALFile: "/tmp/a", HAKeepaliveInterval: 16 * time.Second}
	_, err := New(ctx, config, nil)
	assert.Error(t, err)

	c, err := New(ctx, config, t)
	assert.NoError(t, err)

	c.Finalizer()
	privFile, err := os.CreateTemp("", "priv")
	assert.NoError(t, err)
	_, err = privFile.WriteString("privkey")
	assert.NoError(t, err)
	defer privFile.Close()

	config.Git = types.GitConfig{PrivateKey: privFile.Name()}

	config1 := types.Config{
		WALFile: "/tmp/b",
		Git: types.GitConfig{
			SCMType:    "gitlab",
			PrivateKey: privFile.Name(),
		},
		HAKeepaliveInterval: 16 * time.Second,
	}
	c1, err := New(ctx, config1, t)
	assert.NoError(t, err)
	c1.Finalizer()

	config2 := types.Config{
		WALFile: "/tmp/c",
		Git: types.GitConfig{
			SCMType:    "github",
			PrivateKey: privFile.Name(),
		},
		HAKeepaliveInterval: 16 * time.Second,
	}
	c2, err := New(ctx, config2, t)
	assert.NoError(t, err)
	c2.Finalizer()
}

func TestFinalizer(t *testing.T) {
	c := NewTestCluster()
	store := c.store.(*storemocks.Store)
	store.On("TerminateEmbededStorage").Return(nil)
	c.Finalizer()
}
