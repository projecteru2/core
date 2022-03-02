package calcium

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/resources"
	resourcemocks "github.com/projecteru2/core/resources/mocks"
	sourcemocks "github.com/projecteru2/core/source/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func NewTestCluster() *Calcium {
	walDir, err := ioutil.TempDir(os.TempDir(), "core.wal.*")
	if err != nil {
		panic(err)
	}

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
		GRPCConfig: types.GRPCConfig{
			ServiceDiscoveryPushInterval: 15 * time.Second,
		},
		WALFile:             filepath.Join(walDir, "core.wal.log"),
		MaxConcurrency:      10,
		HAKeepaliveInterval: 16 * time.Second,
	}
	c.store = &storemocks.Store{}
	c.source = &sourcemocks.Source{}
	c.wal = &WAL{WAL: &walmocks.WAL{}}

	mwal := c.wal.WAL.(*walmocks.WAL)
	commit := wal.Commit(func() error { return nil })
	mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)

	plugin := &resourcemocks.Plugin{}
	plugin.On("Name").Return("mock-plugin")
	if c.resource, err = resources.NewPluginManager(c.config); err != nil {
		panic(err)
	}
	c.resource.AddPlugins(plugin)

	return c
}

func TestNewCluster(t *testing.T) {
	config := types.Config{WALFile: "/tmp/a", HAKeepaliveInterval: 16 * time.Second}
	_, err := New(config, nil)
	assert.Error(t, err)

	c, err := New(config, t)
	assert.NoError(t, err)

	c.Finalizer()
	privFile, err := ioutil.TempFile("", "priv")
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
	c1, err := New(config1, t)
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
	c2, err := New(config2, t)
	assert.NoError(t, err)
	c2.Finalizer()
}

func TestFinalizer(t *testing.T) {
	c := NewTestCluster()
	store := &storemocks.Store{}
	c.store = store
	store.On("TerminateEmbededStorage").Return(nil)
	c.Finalizer()
}
