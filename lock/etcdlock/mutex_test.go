package etcdlock

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/core/store/etcdv3/embedded"

	"github.com/stretchr/testify/assert"
)

func TestMutex(t *testing.T) {
	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	cli := cluster.Client("/test")

	_, err = New(cli, "", time.Second*1)
	assert.Error(t, err)
	mutex, err := New(cli, "test", time.Second*1)
	assert.NoError(t, err)

	ctx := context.Background()
	ctx, err = mutex.Lock(ctx)
	assert.Nil(t, ctx.Err())
	assert.NoError(t, err)
	err = mutex.Unlock(ctx)
	assert.NoError(t, err)
	assert.NoError(t, ctx.Err())

	m2, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m2.Lock(context.Background())
	m3, err := New(cli, "test", 100*time.Millisecond)
	assert.NoError(t, err)
	_, err = m3.Lock(context.Background())
	assert.EqualError(t, err, "context deadline exceeded")
	m2.Unlock(context.Background())
	m3.Unlock(context.Background())

	m4, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rCtx, err := m4.Lock(ctx)
	<-rCtx.Done()
	assert.EqualError(t, rCtx.Err(), "context deadline exceeded")
	m4.Unlock(context.Background())

	m5, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m5.Lock(context.Background())
	assert.NoError(t, err)
}
