package calcium

import (
	"fmt"
	"strings"

	"gitlab.ricebook.net/platform/core/network"
	"gitlab.ricebook.net/platform/core/network/calico"
	"gitlab.ricebook.net/platform/core/scheduler"
	"gitlab.ricebook.net/platform/core/scheduler/complex"
	"gitlab.ricebook.net/platform/core/source"
	"gitlab.ricebook.net/platform/core/source/github"
	"gitlab.ricebook.net/platform/core/source/gitlab"
	"gitlab.ricebook.net/platform/core/store"
	"gitlab.ricebook.net/platform/core/store/etcd"
	"gitlab.ricebook.net/platform/core/types"
)

type calcium struct {
	store     store.Store
	config    types.Config
	scheduler scheduler.Scheduler
	network   network.Network
	source    source.Source
}

const (
	AFTER_START = "after_start"
	BEFORE_STOP = "before_stop"
	GITLAB      = "gitlab"
	GITHUB      = "github"
)

// New returns a new cluster config
func New(config types.Config) (*calcium, error) {
	var err error
	store, err := etcdstore.New(config)
	if err != nil {
		return nil, err
	}

	scheduler, err := complexscheduler.New(config)
	if err != nil {
		return nil, err
	}
	titanium := calico.New()

	var scm source.Source
	scmtype := strings.ToLower(config.Git.SCMType)

	switch scmtype {
	case GITLAB:
		scm = gitlab.New(config.Git)
	case GITHUB:
		scm = github.New(config.Git)
	default:
		return nil, fmt.Errorf("Unkonwn SCM type: %s", config.Git.SCMType)
	}

	return &calcium{store: store, config: config, scheduler: scheduler, network: titanium, source: scm}, nil
}

func (c *calcium) SetStore(s store.Store) {
	c.store = s
}
