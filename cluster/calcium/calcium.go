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
	"github.com/projecteru2/core/resource3"
	"github.com/projecteru2/core/resource3/cobalt"
	""
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
	rmgr2      resource3.Manager
	source     source.Source
	watcher    discovery.Service
	wal        wal.WAL
	pool       *ants.PoolWithFunc
	identifier string
}

// New returns a new cluster config
func New(ctx context.Context, config types.Config, t *testing.T) (*Calcium, error) {
	logger := log.WithFunc("calcium.New")
	// set store
	store, err := store.NewStore(config, t)
	if err != nil {
		logger.Error(ctx, err)
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
		logger.Warn(ctx, "SCM not set, build API disabled")
	}
	if err != nil {
		logger.Error(ctx, err, "SCM failed")
		return nil, err
	}

	// set watcher
	watcher := helium.New(ctx, config.GRPCConfig, store)

	// set resource plugin manager
	rmgr2, err := cobalt.New(config)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	if err := rmgr2.LoadPlugins(ctx); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	// init pool
	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}

	cal := &Calcium{store: store, config: config, source: scm, watcher: watcher, rmgr2: rmgr2, pool: pool}

	cal.wal, err = enableWAL(config, cal, store)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	cal.identifier, err = config.Identifier()
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	return cal, pool.Invoke(func() { cal.InitMetrics(ctx) })
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
