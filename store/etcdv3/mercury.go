package etcdv3

import (
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/etcdv3/meta"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Mercury is the etcd backed store.
type Mercury struct {
	*common.Store
	meta.KV

	config types.Config
	pool   *ants.PoolWithFunc
}

// New creates a Mercury on the given etcd cluster.
func New(config types.Config, embeddedETCD *embedded.Cluster) (*Mercury, error) {
	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}
	kv, err := meta.NewETCD(config.Etcd, embeddedETCD)
	if err != nil {
		return nil, err
	}
	return &Mercury{
		Store:  common.New(&etcdKV{kv: kv}, config, pool),
		KV:     kv,
		config: config,
		pool:   pool,
	}, nil
}
