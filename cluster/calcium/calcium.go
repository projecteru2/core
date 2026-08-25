package calcium

import (
	"context"
	"strings"

	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/discovery"
	"github.com/projecteru2/core/discovery/helium"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource"
	"github.com/projecteru2/core/resource/cobalt"
	"github.com/projecteru2/core/source"
	"github.com/projecteru2/core/source/github"
	"github.com/projecteru2/core/source/gitlab"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	storefactory "github.com/projecteru2/core/store/factory"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

// Calcium implements the cluster.Cluster interface.
type Calcium struct {
	config     types.Config
	store      store.Store
	rmgr       resource.Manager
	source     source.Source
	watcher    discovery.Service
	wal        wal.WAL
	pool       *ants.PoolWithFunc
	identifier string
	// serviceAddress is both the key RegisterService publishes and the journal's own prefix
	serviceAddress string
}

// New returns a Calcium cluster.
func New(ctx context.Context, config types.Config, embeddedETCD *embedded.Cluster) (*Calcium, error) {
	logger := log.WithFunc("calcium.New")
	store, err := storefactory.NewStore(config, embeddedETCD)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

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

	watcher := helium.New(ctx, config.GRPCConfig, store)

	rmgr, err := cobalt.New(config)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	if err = rmgr.LoadPlugins(ctx, embeddedETCD); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}

	cal := &Calcium{store: store, config: config, source: scm, watcher: watcher, rmgr: rmgr, pool: pool}

	cal.serviceAddress, err = utils.GetOutboundAddress(config.Bind, config.ProbeTarget)
	if err != nil {
		logger.Error(ctx, err, "failed to get outbound address")
		return nil, err
	}

	cal.wal, err = enableWAL(ctx, config, cal, store)
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

// DisasterRecover replays the WAL to finish interrupted writes.
func (c *Calcium) DisasterRecover(ctx context.Context) {
	c.wal.Recover(ctx)
}

func (c *Calcium) Finalizer() {
	ctx := context.Background()
	if err := c.wal.Close(); err != nil {
		log.WithFunc("calcium.Finalizer").Error(ctx, err, "close wal")
	}
	c.pool.Release()
}

func (c *Calcium) GetIdentifier() string {
	return c.identifier
}

func (c *Calcium) GetStore() store.Store {
	return c.store
}
