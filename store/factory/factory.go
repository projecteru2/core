package factory

import (
	"context"

	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/store/redis"
	"github.com/projecteru2/core/types"
)

// NewStore creates the store backend named by config.
func NewStore(ctx context.Context, config types.Config, embeddedETCD *embedded.Cluster) (store.Store, error) {
	if config.Store == types.Redis {
		return redis.New(config)
	}
	return etcdv3.New(ctx, config, embeddedETCD)
}
