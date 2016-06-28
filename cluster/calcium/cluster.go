package calcium

import (
	"sync"

	"gitlab.ricebook.net/platform/core/network"
	"gitlab.ricebook.net/platform/core/network/calico"
	"gitlab.ricebook.net/platform/core/scheduler"
	"gitlab.ricebook.net/platform/core/scheduler/simple"
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
}

func New(config types.Config) (*Calcium, error) {
	store, err := etcdstore.NewKrypton(config)
	if err != nil {
		return nil, err
	}

	scheduler := &simplescheduler.Magnesium{}
	titanium := &calico.Titanium{}

	return &Calcium{store: store, config: config, scheduler: scheduler, network: titanium}, nil
}
