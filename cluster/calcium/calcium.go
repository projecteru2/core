package calcium

import (
	"context"
	"strings"

	"github.com/pkg/errors"
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
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/store/redis"
	"github.com/projecteru2/core/types"
)

// Calcium implement the cluster
type Calcium struct {
	config    types.Config
	store     store.Store
	scheduler scheduler.Scheduler
	source    source.Source
	watcher   discovery.Service
	wal       *WAL
}

// New returns a new cluster config
func New(config types.Config, embeddedStorage bool) (*Calcium, error) {
	logger := log.WithField("Calcium", "New").WithField("config", config)

	// set store
	var store store.Store
	var err error
	switch config.Store {
	case types.Etcd:
		store, err = etcdv3.New(config, embeddedStorage)
		if err != nil {
			return nil, logger.Err(errors.WithStack(err))
		}
	case types.Redis:
		store, err = redis.New(config, embeddedStorage)
		if err != nil {
			return nil, logger.Err(errors.WithStack(err))
		}
	}

	// set scheduler
	potassium, err := complexscheduler.New(config)
	if err != nil {
		return nil, logger.Err(errors.WithStack(err))
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
		logger.Errorf("[Calcium] SCAM failed: %+v", err)
		return nil, errors.WithStack(err)
	}

	// set watcher
	watcher := helium.New(config.GRPCConfig, store)

	cal := &Calcium{store: store, config: config, scheduler: potassium, source: scm, watcher: watcher}
	cal.wal, err = newCalciumWAL(cal)
	return cal, logger.Err(errors.WithStack(err))
}

// DisasterRecover .
func (c *Calcium) DisasterRecover(ctx context.Context) {
	c.wal.Recover(ctx)
}

// Finalizer use for defer
func (c *Calcium) Finalizer() {
	c.store.TerminateEmbededStorage()
}
