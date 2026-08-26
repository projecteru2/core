package calcium

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	resourcemocks "github.com/projecteru2/core/resource/mocks"
	sourcemocks "github.com/projecteru2/core/source/mocks"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestNewCluster(t *testing.T) {
	ctx := context.Background()
	config := types.Config{Bind: ":5001", ProbeTarget: "8.8.8.8:80", HAKeepaliveInterval: 16 * time.Second}
	_, err := New(ctx, config, nil)
	assert.Error(t, err)

	embeddedETCD, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(embeddedETCD.Close)
	c, err := New(ctx, config, embeddedETCD)
	assert.NoError(t, err)

	c.Finalizer()
	privFile, err := os.CreateTemp("", "priv")
	assert.NoError(t, err)
	_, err = privFile.WriteString("privkey")
	assert.NoError(t, err)
	defer privFile.Close()

	config.Git = types.GitConfig{PrivateKey: privFile.Name()}

	config1 := types.Config{
		Bind:        ":5001",
		ProbeTarget: "8.8.8.8:80",
		Git: types.GitConfig{
			SCMType:    "gitlab",
			PrivateKey: privFile.Name(),
		},
		HAKeepaliveInterval: 16 * time.Second,
	}
	c1, err := New(ctx, config1, embeddedETCD)
	assert.NoError(t, err)
	c1.Finalizer()

	config2 := types.Config{
		Bind:        ":5001",
		ProbeTarget: "8.8.8.8:80",
		Git: types.GitConfig{
			SCMType:    "github",
			PrivateKey: privFile.Name(),
		},
		HAKeepaliveInterval: 16 * time.Second,
	}
	c2, err := New(ctx, config2, embeddedETCD)
	assert.NoError(t, err)
	c2.Finalizer()
}

func TestFinalizer(t *testing.T) {
	NewTestCluster().Finalizer()
}

func NewTestCluster() *Calcium {
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
			MaxRecvMsgSize:               20971520,
			ServiceDiscoveryPushInterval: 15 * time.Second,
		},
		MaxConcurrency:      100000,
		HAKeepaliveInterval: 16 * time.Second,
		ProbeTarget:         "8.8.8.8:80",
		Bind:                ":5001",
	}
	mwal := &walmocks.WAL{}
	commit := wal.Commit(func() error { return nil })
	mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)
	mwal.On("Close").Return(nil)

	c.store = &storemocks.Store{}
	c.source = &sourcemocks.Source{}
	c.rmgr = &resourcemocks.Manager{}
	c.wal = mwal

	return c
}
