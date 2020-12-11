package calcium

import (
	"strings"

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
	"github.com/projecteru2/core/types"
)

// Calcium implement the cluster
type Calcium struct {
	config    types.Config
	store     store.Store
	scheduler scheduler.Scheduler
	source    source.Source
	watcher   discovery.Service
}

// New returns a new cluster config
func New(config types.Config, embeddedStorage bool) (*Calcium, error) {
	// set store
	store, err := etcdv3.New(config, embeddedStorage)
	if err != nil {
		return nil, err
	}

	// set scheduler
	potassium, err := complexscheduler.New(config)
	if err != nil {
		return nil, err
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

	// set watcher
	watcher := helium.New(config.GRPCConfig, store)

	return &Calcium{store: store, config: config, scheduler: potassium, source: scm, watcher: watcher}, err
}

// Finalizer use for defer
func (c *Calcium) Finalizer() {
	c.store.TerminateEmbededStorage()
}
