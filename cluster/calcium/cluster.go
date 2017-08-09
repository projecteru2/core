package calcium

import (
	"fmt"
	"strings"
	"time"

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
	// set timeout config
	config = setTimeout(config)

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
	titanium := calico.New(config)

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

func setTimeout(config types.Config) types.Config {
	ct := config.Timeout

	ct.Common *= time.Second
	c := ct.Common
	if ct.Backup *= time.Second; ct.Backup == 0 {
		ct.Backup = c
	}
	if ct.BuildImage *= time.Second; ct.BuildImage == 0 {
		ct.BuildImage = c
	}
	if ct.CreateContainer *= time.Second; ct.CreateContainer == 0 {
		ct.CreateContainer = c
	}
	if ct.RemoveContainer *= time.Second; ct.RemoveContainer == 0 {
		ct.RemoveContainer = c
	}
	if ct.RemoveImage *= time.Second; ct.RemoveImage == 0 {
		ct.RemoveImage = c
	}
	if ct.RunAndWait *= time.Second; ct.RunAndWait == 0 {
		ct.RunAndWait = c
	}

	config.Timeout = ct
	return config
}
