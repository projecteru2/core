package etcdlock

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/core/store/etcdv3/embedded"
	"github.com/stretchr/testify/assert"
)

func TestMutex(t *testing.T) {
	embedd := embedded.NewCluster(t, "/test")
	cli := embedd.RandClient()

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
	embedd := embedded.NewCluster(t, "/test")
	cli := embedd.RandClient()

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
