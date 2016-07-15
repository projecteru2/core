package calcium

import (
	"fmt"

	"gitlab.ricebook.net/platform/core/network"
	"gitlab.ricebook.net/platform/core/network/calico"
	"gitlab.ricebook.net/platform/core/scheduler"
	"gitlab.ricebook.net/platform/core/scheduler/complex"
	"gitlab.ricebook.net/platform/core/scheduler/simple"
	"gitlab.ricebook.net/platform/core/source"
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

func New(config types.Config) (*calcium, error) {
	store, err := etcdstore.New(config)
	if err != nil {
		return nil, err
	}

	var scheduler scheduler.Scheduler
	if config.Scheduler.Type == "simple" {
		scheduler = simplescheduler.New()
	} else if config.Scheduler.Type == "complex" {
		scheduler, err = complexscheduler.New(config)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("Wrong type for scheduler: either simple or complex")
	}

	titanium := calico.New()
	source := gitlab.New(config)

	return &calcium{store: store, config: config, scheduler: scheduler, network: titanium, source: source}, nil
}
