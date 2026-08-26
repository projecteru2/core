package etcdlock

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/store/etcdv3/embedded"
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

	ctx := t.Context()
	ctx, err = mutex.Lock(ctx)
	assert.Nil(t, ctx.Err())
	assert.NoError(t, err)
	err = mutex.Unlock(ctx)
	assert.NoError(t, err)
	assert.NoError(t, ctx.Err())

	m2, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m2.Lock(t.Context())
	m3, err := New(cli, "test", 100*time.Millisecond)
	assert.NoError(t, err)
	_, err = m3.Lock(t.Context())
	assert.EqualError(t, err, "context deadline exceeded")
	m2.Unlock(t.Context())
	m3.Unlock(t.Context())

	m4, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	rCtx, err := m4.Lock(ctx)
	<-rCtx.Done()
	assert.EqualError(t, rCtx.Err(), "context deadline exceeded")
	m4.Unlock(t.Context())

	m5, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m5.Lock(t.Context())
	assert.NoError(t, err)
}
