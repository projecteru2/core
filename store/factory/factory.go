package factory

import (
	"testing"

	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/store/redis"
	"github.com/projecteru2/core/types"
)

// NewStore creates a store
func NewStore(config types.Config, t *testing.T) (stor store.Store, err error) {
	switch config.Store {
	case types.Redis:
		stor, err = redis.New(config, t)
	default:
		stor, err = etcdv3.New(config, t)
	}
	return stor, err
}
