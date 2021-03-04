package kv

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMockedKV(t *testing.T) {
	m := &MockedKV{}
	require.NoError(t, m.Open(context.Background(), "/tmp/wal", 0777, time.Second))

	a := []byte("/a")
	b := []byte("/b")
	expValue := []byte("v")
	ctx := context.Background()
	require.NoError(t, m.Put(ctx, a, expValue))
	require.NoError(t, m.Put(ctx, b, expValue))
	require.NoError(t, m.Put(ctx, []byte("out-of-scan"), expValue))

	ch, abort := m.Scan(ctx, []byte("/"))
	elem := <-ch
	ent, ok := elem.(MockedScanEntry)
	require.True(t, ok)
	require.NoError(t, ent.Err)
	abort()

	_, abort = m.Scan(ctx, []byte("/"))
	abort()

	realValue, err := m.Get(ctx, a)
	require.NoError(t, err)
	require.Equal(t, expValue, realValue)

	realValue, err = m.Get(ctx, b)
	require.NoError(t, err)
	require.Equal(t, expValue, realValue)

	require.NoError(t, m.Delete(ctx, b))
	realValue, err = m.Get(ctx, b)
	require.Error(t, err)

	require.NoError(t, m.Close(ctx))

	realValue, err = m.Get(ctx, a)
	require.Error(t, err)
}
