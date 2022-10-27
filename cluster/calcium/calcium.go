package calcium

import (
	"context"
	"strings"
	"testing"

	"github.com/panjf2000/ants/v2"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/discovery"
	"github.com/projecteru2/core/discovery/helium"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/resources/cpumem"
	"github.com/projecteru2/core/resources/volume"
	"github.com/projecteru2/core/source"
	"github.com/projecteru2/core/source/github"
	"github.com/projecteru2/core/source/gitlab"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

// Calcium implement the cluster
type Calcium struct {
	config     types.Config
	store      store.Store
	rmgr       resources.Manager
	source     source.Source
	watcher    discovery.Service
	wal        wal.WAL
	pool       *ants.PoolWithFunc
	identifier string
}

// New returns a new cluster config
func New(ctx context.Context, config types.Config, t *testing.T) (*Calcium, error) {
	// set store
	store, err := store.NewStore(config, t)
	if err != nil {
		log.Error(ctx, err)
		return nil, err
	}

	// set scm
	var scm source.Source
	scmtype := strings.ToLower(config.Git.SCMType)
	switch scmtype {
	case cluster.Gitlab:
		scm, err = gitlab.New(config)
	case cluster.Github:
		scm, err = github.New(config)
	default:
		log.Warn(ctx, "[Calcium] SCM not set, build API disabled")
	}
	if err != nil {
		log.Errorf(ctx, err, "[Calcium] SCM failed: %+v", err)
		return nil, err
	}

	// set watcher
	watcher := helium.New(config.GRPCConfig, store)

	// set resource plugin manager
	rmgr, err := resources.NewPluginsManager(config)
	if err != nil {
		log.Error(ctx, err)
		return nil, err
	}

	// load internal plugins
	cpumem, err := cpumem.NewPlugin(config)
	if err != nil {
		log.Errorf(ctx, err, "[NewPluginManager] new cpumem plugin error: %v", err)
		return nil, err
	}
	volume, err := volume.NewPlugin(config)
	if err != nil {
		log.Errorf(ctx, err, "[NewPluginManager] new volume plugin error: %v", err)
		return nil, err
	}
	rmgr.AddPlugins(cpumem, volume)
	// load binary plugins
	if err = rmgr.LoadPlugins(ctx); err != nil {
		return nil, err
	}

	// init pool
	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}

	cal := &Calcium{store: store, config: config, source: scm, watcher: watcher, rmgr: rmgr, pool: pool}

	cal.wal, err = enableWAL(config, cal, store)
	if err != nil {
		log.Error(ctx, err)
		return nil, err
	}

	cal.identifier, err = config.Identifier()
	if err != nil {
		log.Error(ctx, err)
		return nil, err
	}

	_ = pool.Invoke(func() { cal.InitMetrics(ctx) })
	log.Error(ctx, err)
	return cal, err
}

// DisasterRecover .
func (c *Calcium) DisasterRecover(ctx context.Context) {
	c.wal.Recover(ctx)
}

// Finalizer use for defer
func (c *Calcium) Finalizer() {
	// TODO some resource recycle
	c.pool.Release()
}

// GetIdentifier returns the identifier of calcium
func (c *Calcium) GetIdentifier() string {
	return c.identifier
}
