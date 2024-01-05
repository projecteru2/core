package factory

import (
	"testing"

	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/store/redis"
	"github.com/projecteru2/core/types"
)

// NewStore creates a store
func NewStore(config types.Config, t *testing.T) (sto store.Store, err error) {
	switch config.Store {
	case types.Redis:
		sto, err = redis.New(config, t)
	default:
		sto, err = etcdv3.New(config, t)
	}
	return sto, err
}
