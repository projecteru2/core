package etcdlock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.etcd.io/etcd/v3/integration"
)

func TestMutex(t *testing.T) {
	cluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer cluster.Terminate(t)
	cli := cluster.RandClient()

	_, err := New(cli, "", time.Second*1)
	assert.Error(t, err)
	mutex, err := New(cli, "test", time.Second*1)
	assert.NoError(t, err)

	ctx := context.Background()
	ctx, err = mutex.Lock(ctx)
	assert.Nil(t, ctx.Err())
	assert.NoError(t, err)
	err = mutex.Unlock(ctx)
	assert.NoError(t, err)
	assert.EqualError(t, ctx.Err(), "lock session done")
}

func TestTryLock(t *testing.T) {
	cluster := integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
	defer cluster.Terminate(t)
	cli := cluster.RandClient()

	m1, err := New(cli, "test", time.Second*1)
	assert.NoError(t, err)
	m2, err := New(cli, "test", time.Second*1)
	assert.NoError(t, err)

	ctx1, err := m1.Lock(context.Background())
	assert.Nil(t, ctx1.Err())
	assert.NoError(t, err)

	ctx2, err := m2.TryLock(context.Background())
	assert.Nil(t, ctx2)
	assert.Error(t, err)
}
