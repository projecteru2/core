package calcium

import (
	"gitlab.ricebook.net/platform/core/network"
	"gitlab.ricebook.net/platform/core/network/calico"
	"gitlab.ricebook.net/platform/core/scheduler"
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

const (
	AFTER_START = "after_start"
	BEFORE_STOP = "before_stop"
)

func New(config types.Config) (*calcium, error) {
	var err error
	store, err := etcdstore.New(config)
	if err != nil {
		return nil, err
	}

	// TODO 这里必须要用到scheduler, 要用丫的RandomNode方法
	// 所以先给个simple吧
	scheduler := simplescheduler.New()
	titanium := calico.New()
	source := gitlab.New(config)

	return &calcium{store: store, config: config, scheduler: scheduler, network: titanium, source: source}, nil
}
