package calcium

import (
	"context"
	"strings"
	"testing"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/discovery"
	"github.com/projecteru2/core/discovery/helium"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/scheduler"
	complexscheduler "github.com/projecteru2/core/scheduler/complex"
	"github.com/projecteru2/core/source"
	"github.com/projecteru2/core/source/github"
	"github.com/projecteru2/core/source/gitlab"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
)

// Calcium implement the cluster
type Calcium struct {
	config     types.Config
	store      store.Store
	scheduler  scheduler.Scheduler
	source     source.Source
	watcher    discovery.Service
	wal        *WAL
	identifier string
}

// New returns a new cluster config
func New(config types.Config, t *testing.T) (*Calcium, error) {
	logger := log.WithField("Calcium", "New").WithField("config", config)

	// set store
	store, err := store.NewStore(config, t)
	if err != nil {
		return nil, logger.Err(context.TODO(), errors.WithStack(err))
	}

	// set scheduler
	potassium, err := complexscheduler.New(config)
	if err != nil {
		return nil, logger.Err(context.TODO(), errors.WithStack(err))
	}
	scheduler.InitSchedulerV1(potassium)

	// set scm
	var scm source.Source
	scmtype := strings.ToLower(config.Git.SCMType)
	switch scmtype {
	case cluster.Gitlab:
		scm, err = gitlab.New(config)
	case cluster.Github:
		scm, err = github.New(config)
	default:
		log.Warn("[Calcium] SCM not set, build API disabled")
	}
	if err != nil {
		logger.Errorf(nil, "[Calcium] SCM failed: %+v", err) //nolint
		return nil, errors.WithStack(err)
	}

	// set watcher
	watcher := helium.New(config.GRPCConfig, store)

	cal := &Calcium{store: store, config: config, scheduler: potassium, source: scm, watcher: watcher}
	cal.wal, err = newWAL(config, cal)
	cal.identifier = config.Identifier()

	return cal, logger.Err(nil, errors.WithStack(err)) //nolint
}

// DisasterRecover .
func (c *Calcium) DisasterRecover(ctx context.Context) {
	c.wal.Recover(ctx)
}

// Finalizer use for defer
func (c *Calcium) Finalizer() {
	// TODO some resource recycle
}

// GetIdentifier returns the identifier of calcium
func (c *Calcium) GetIdentifier() string {
	return c.identifier
}
