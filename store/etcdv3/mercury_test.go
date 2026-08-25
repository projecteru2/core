package etcdv3

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func NewMercury(t *testing.T) *Mercury {
	config := types.Config{}
	config.LockTimeout = 10 * time.Second
	config.GlobalTimeout = 30 * time.Second
	config.Etcd = types.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	config.ProbeTarget = "8.8.8.8:80"
	config.MaxConcurrency = 100000

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	factory.InitEngineCache(ctx, config, nil)

	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	m, err := New(config, cluster)
	assert.NoError(t, err)
	return m
}
