package etcdlock

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/coreos/etcd/integration"
)

func TestMutex(t *testing.T) {
	cluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer cluster.Terminate(t)
	cli := cluster.RandClient()

	_, err := New(cli, "", 1)
	assert.Error(t, err)
	mutex, err := New(cli, "test", 1)
	assert.NoError(t, err)

	ctx := context.Background()
	err = mutex.Lock(ctx)
	assert.NoError(t, err)
	err = mutex.Unlock(ctx)
	assert.NoError(t, err)
}
