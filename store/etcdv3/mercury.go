package etcdv3

import (
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/etcdv3/meta"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Mercury is the etcd backed store.
type Mercury struct {
	*common.Store

	kv meta.KV
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
	return &Mercury{Store: common.New(&etcdKV{kv: kv}, config, pool), kv: kv}, nil
}
