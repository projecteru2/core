package etcdv3

import (
	"github.com/panjf2000/ants/v2"

	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/etcdv3/meta"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Mercury is the etcd backed store.
type Mercury struct {
	meta.KV
	config types.Config
	pool   *ants.PoolWithFunc
}

// New creates a Mercury on the given etcd cluster.
func New(config types.Config, embeddedETCD *embedded.Cluster) (m *Mercury, err error) {
	pool, err := utils.NewPool(config.MaxConcurrency)
	if err != nil {
		return nil, err
	}
	m = &Mercury{config: config, pool: pool}
	m.KV, err = meta.NewETCD(config.Etcd, embeddedETCD)
	return m, err
}
