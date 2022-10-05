package kv

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestMockedKV(t *testing.T) {
	m := NewMockedKV()
	require.NoError(t, m.Open("/tmp/wal", 0777, time.Second))

	a := []byte("/a")
	b := []byte("/b")
	expValue := []byte("v")
	require.NoError(t, m.Put(a, expValue))
	require.NoError(t, m.Put(b, expValue))
	require.NoError(t, m.Put([]byte("out-of-scan"), expValue))

	ch, abort := m.Scan([]byte("/"))
	elem := <-ch
	ent, ok := elem.(MockedScanEntry)
	require.True(t, ok)
	require.NoError(t, ent.Err)
	abort()

	_, abort = m.Scan([]byte("/"))
	abort()

	realValue, err := m.Get(a)
	require.NoError(t, err)
	require.Equal(t, expValue, realValue)

	realValue, err = m.Get(b)
	require.NoError(t, err)
	require.Equal(t, expValue, realValue)

	require.NoError(t, m.Delete(b))
	realValue, err = m.Get(b)
	require.Error(t, err)

	require.NoError(t, m.Close())

	realValue, err = m.Get(a)
	require.Error(t, err)
}
