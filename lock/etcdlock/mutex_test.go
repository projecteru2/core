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
	pool := NewPool(cluster.Client("/test"), time.Second)

	_, err = New(pool, "", time.Second*1)
	assert.Error(t, err)
	mutex, err := New(pool, "test", time.Second*1)
	assert.NoError(t, err)

	ctx := t.Context()
	ctx, err = mutex.Lock(ctx)
	assert.Nil(t, ctx.Err())
	assert.NoError(t, err)
	err = mutex.Unlock(ctx)
	assert.NoError(t, err)
	assert.NoError(t, ctx.Err())

	m2, err := New(pool, "test", time.Second)
	assert.NoError(t, err)
	_, err = m2.Lock(t.Context())
	m3, err := New(pool, "test", 100*time.Millisecond)
	assert.NoError(t, err)
	_, err = m3.Lock(t.Context())
	assert.EqualError(t, err, "context deadline exceeded")
	m2.Unlock(t.Context())
	m3.Unlock(t.Context())

	m4, err := New(pool, "test", time.Second)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()
	rCtx, err := m4.Lock(ctx)
	<-rCtx.Done()
	assert.EqualError(t, rCtx.Err(), "context deadline exceeded")
	m4.Unlock(t.Context())

	m5, err := New(pool, "test", time.Second)
	assert.NoError(t, err)
	_, err = m5.Lock(t.Context())
	assert.NoError(t, err)
}

func TestPoolReusesTheSessionOfAnUnlockedMutex(t *testing.T) {
	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	pool := NewPool(cluster.Client("/test"), time.Second)

	first, err := New(pool, "reuse", time.Second)
	assert.NoError(t, err)
	_, err = first.Lock(t.Context())
	assert.NoError(t, err)
	session := first.session
	assert.NoError(t, first.Unlock(t.Context()))

	second, err := New(pool, "reuse", time.Second)
	assert.NoError(t, err)
	assert.Same(t, session, second.session)
	_, err = second.Lock(t.Context())
	assert.NoError(t, err)
	assert.NoError(t, second.Unlock(t.Context()))
	assert.Len(t, pool.idle, 1)
}

func TestMutexesOnOneKeyHoldDistinctSessions(t *testing.T) {
	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	pool := NewPool(cluster.Client("/test"), time.Second)

	holder, err := New(pool, "distinct", time.Second)
	assert.NoError(t, err)
	_, err = holder.Lock(t.Context())
	assert.NoError(t, err)

	waiter, err := New(pool, "distinct", 100*time.Millisecond)
	assert.NoError(t, err)
	assert.NotSame(t, holder.session, waiter.session)
	_, err = waiter.Lock(t.Context())
	assert.EqualError(t, err, "context deadline exceeded")
	assert.NoError(t, holder.Unlock(t.Context()))
}

func TestLockFailureRevokesTheSession(t *testing.T) {
	cluster, err := embedded.New(t.TempDir())
	assert.NoError(t, err)
	t.Cleanup(cluster.Close)
	cli := cluster.Client("/test")
	pool := NewPool(cli, time.Second)

	holder, err := New(pool, "revoke", time.Second)
	assert.NoError(t, err)
	_, err = holder.Lock(t.Context())
	assert.NoError(t, err)

	waiter, err := New(pool, "revoke", 100*time.Millisecond)
	assert.NoError(t, err)
	_, err = waiter.Lock(t.Context())
	assert.EqualError(t, err, "context deadline exceeded")

	leases, err := cli.Leases(t.Context())
	assert.NoError(t, err)
	assert.Len(t, leases.Leases, 1)
	assert.Empty(t, pool.idle)
	assert.NoError(t, waiter.Unlock(t.Context()))
	assert.NoError(t, holder.Unlock(t.Context()))
}
