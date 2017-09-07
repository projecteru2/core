package calcium

import (
	"fmt"
	"strings"

	"github.com/projecteru2/core/network"
	"github.com/projecteru2/core/network/calico"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/scheduler/complex"
	"github.com/projecteru2/core/source"
	"github.com/projecteru2/core/source/github"
	"github.com/projecteru2/core/source/gitlab"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/store/etcd"
	"github.com/projecteru2/core/types"
)

type calcium struct {
	store     store.Store
	config    types.Config
	scheduler scheduler.Scheduler
	network   network.Network
	source    source.Source
}

const (
	afterStart = "after_start"
	beforeStop = "before_stop"
	GITLAB     = "gitlab"
	GITHUB     = "github"
)

// New returns a new cluster config
func New(config types.Config) (*calcium, error) {
	// set store
	store, err := etcdstore.New(config)
	if err != nil {
		return nil, err
	}

	// set scheduler
	scheduler, err := complexscheduler.New(config)
	if err != nil {
		return nil, err
	}

	// set network
	titanium := calico.New()

	// set scm
	var scm source.Source
	scmtype := strings.ToLower(config.Git.SCMType)
	switch scmtype {
	case GITLAB:
		scm = gitlab.New(config)
	case GITHUB:
		scm = github.New(config)
	default:
		return nil, fmt.Errorf("Unkonwn SCM type: %s", config.Git.SCMType)
	}

	return &calcium{store: store, config: config, scheduler: scheduler, network: titanium, source: scm}, nil
}

// SetStore 用于在单元测试中更改etcd store为mock store
func (c *calcium) SetStore(s store.Store) {
	c.store = s
}
