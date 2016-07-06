package calcium

import (
	"sync"

	"github.com/coreos/etcd/client"
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

type Calcium struct {
	sync.Mutex
	store     store.Store
	config    types.Config
	scheduler scheduler.Scheduler
	network   network.Network
	source    source.Source
}

func New(config types.Config) (*Calcium, error) {
	store, err := etcdstore.NewKrypton(config)
	if err != nil {
		return nil, err
	}

	if config.Scheduler == "simple" {
		scheduler := simplescheduler.New()
	} else {
		scheduler := complexscheduler.NewPotassim(store, config.EtcdLockKey, config.EtcdLockTTL)
	}

	titanium := calico.New()
	source := gitlab.New(config)

	return &Calcium{store: store, config: config, scheduler: scheduler, network: titanium, source: source}, nil
}
