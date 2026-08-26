package etcdv3

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"
)

func NewMercury(t *testing.T) *Mercury {
	config := types.Config{}
	config.Etcd = types.EtcdConfig{
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	config.MaxConcurrency = 100000

	factory.InitEngineCache(t.Context(), config, nil)

	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	m, err := New(config, cluster)
	assert.NoError(t, err)
	return m
}
