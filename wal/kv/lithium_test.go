package kv

import (
	"context"
	"fmt"
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

	ch, _ := lit.Scan(context.Background(), []byte("/p1/"))
	require.Equal(t, LithiumScanEntry{key: key, value: value}, <-ch)
	require.Nil(t, <-ch)
}

func TestScanAbort(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	for i := 0; i < 10; i++ {
		key := []byte(fmt.Sprintf("p%d", i))
		require.NoError(t, lit.Put(context.Background(), key, []byte("v")))
	}

	ch, abort := lit.Scan(context.Background(), []byte("p"))
	abort()

	// before the above abort() has been finished, the scanned key/value pair
	// had sent to ch already, then the code tries to recv again to make sure the
	// ch had been closed.
	if real := <-ch; real != nil {
		require.Nil(t, <-ch)
	}
}

func TestNextSequence(t *testing.T) {
	// TODO
}

func TestScanOrderedKeys(t *testing.T) {
	// TODO
}

func newTestLithium(t *testing.T) (lit *Lithium, cancel func()) {
	path := "/tmp/lithium.unitest.wal"
	os.Remove(path)

	lit = NewLithium()
	require.NoError(t, lit.Open(context.Background(), path, 0666, time.Second))

	cancel = func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		closed := make(chan struct{})
		go func() {
			defer close(closed)
			require.NoError(t, lit.Close(ctx))
		}()

		select {
		case <-ctx.Done():
			require.FailNow(t, "close error: %s", ctx.Err())
		case <-closed:
		}
	}

	return
}
