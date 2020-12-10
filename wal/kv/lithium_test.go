package kv

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()
	require.NoError(t, lit.Put(context.Background(), []byte("key"), []byte("value")))
}

func TestGet(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	key := []byte("key")
	value := []byte("value")
	require.NoError(t, lit.Put(context.Background(), key, value))

	act, err := lit.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, value, act)
}

func TestDelete(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	key := []byte("key")
	value := []byte("value")
	require.NoError(t, lit.Put(context.Background(), key, value))

	act, err := lit.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, value, act)

	// deletes the key
	require.NoError(t, lit.Delete(context.Background(), key))

	act, err = lit.Get(context.Background(), key)
	require.NoError(t, err)
	require.Equal(t, []byte{}, act)
}

func TestScan(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	key := []byte("/p1/key")
	value := []byte("value")
	require.NoError(t, lit.Put(context.Background(), key, value))
	require.NoError(t, lit.Put(context.Background(), []byte("/p2/key"), value))

	ch := lit.Scan(context.Background(), []byte("/p1/"))
	require.Equal(t, LithiumScanEntry{key: key, value: value}, <-ch)
	require.Equal(t, nil, <-ch)
}

func TestNextSequence(t *testing.T) {
	// TODO
}

func newTestLithium(t *testing.T) (lit *Lithium, cancel func()) {
	path := "/tmp/lithium.unitest.wal"
	os.Remove(path)

	lit = NewLithium()
	require.NoError(t, lit.Open(context.Background(), path, 0666, time.Second))

	cancel = func() {
		require.NoError(t, lit.Close(context.Background()))
	}

	return
}
