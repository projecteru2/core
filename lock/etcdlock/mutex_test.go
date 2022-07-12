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
	assert.NoError(t, ctx.Err())

	// round 2: another lock attempt timeout

	m2, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m2.Lock(context.TODO())
	m3, err := New(cli, "test", 100*time.Millisecond)
	assert.NoError(t, err)
	_, err = m3.Lock(context.TODO())
	assert.EqualError(t, err, "context deadline exceeded")
	m2.Unlock(context.TODO())
	m3.Unlock(context.TODO())

	// round 3: ctx canceled after lock secured
	m4, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rCtx, err := m4.Lock(ctx)
	<-rCtx.Done()
	assert.EqualError(t, rCtx.Err(), "context deadline exceeded")
	m4.Unlock(context.TODO())

	// round 4: passive release

	m5, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m5.Lock(context.Background())
	assert.NoError(t, err)
	// then after embedded ETCD close, m5 will be unlocked from passive branch
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

	assert.NoError(t, m1.Unlock(context.TODO()))
	assert.NoError(t, m2.Unlock(context.TODO()))

	// round 2: lock conflict

	m3, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	m4, err := New(cli, "test", time.Second)
	assert.NoError(t, err)

	rCtx, err := m3.TryLock(context.TODO())
	assert.NoError(t, err)
	_, err = m4.TryLock(context.TODO())
	assert.EqualError(t, err, "mutex: Locked by another session")
	m4.Unlock(context.TODO())
	m3.Unlock(context.TODO())
	assert.NoError(t, rCtx.Err())

	// round 3: ctx canceled after lock secured
	m5, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	rCtx, err = m5.TryLock(ctx)
	<-rCtx.Done()
	assert.EqualError(t, rCtx.Err(), "context deadline exceeded")
	m5.Unlock(context.TODO())

	// round 4: passive release

	m6, err := New(cli, "test", time.Second)
	assert.NoError(t, err)
	_, err = m6.TryLock(context.Background())
	assert.NoError(t, err)
	// then after embedded ETCD close, m5 will be unlocked from passive branch
}
