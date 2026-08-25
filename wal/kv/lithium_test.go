package kv

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSet(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()
	require.NoError(t, lit.Put([]byte("key"), []byte("value")))
}

func TestGet(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	key := []byte("key")
	value := []byte("value")
	require.NoError(t, lit.Put(key, value))

	act, err := lit.Get(key)
	require.NoError(t, err)
	require.Equal(t, value, act)
}

func TestDelete(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	key := []byte("key")
	value := []byte("value")
	require.NoError(t, lit.Put(key, value))

	act, err := lit.Get(key)
	require.NoError(t, err)
	require.Equal(t, value, act)

	require.NoError(t, lit.Delete(key))

	act, err = lit.Get(key)
	require.NoError(t, err)
	require.Equal(t, []byte{}, act)
}

func TestScan(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	key := []byte("/p1/key")
	value := []byte("value")
	require.NoError(t, lit.Put(key, value))
	require.NoError(t, lit.Put([]byte("/p2/key"), value))

	ch, _ := lit.Scan([]byte("/p1/"))
	require.Equal(t, LithiumScanEntry{key: key, value: value}, <-ch)
	require.Nil(t, <-ch)
}

func TestScanAbort(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	for i := range 10 {
		key := []byte(fmt.Sprintf("p%d", i))
		require.NoError(t, lit.Put(key, []byte("v")))
	}

	ch, abort := lit.Scan([]byte("p"))
	abort()

	if real := <-ch; real != nil {
		require.Nil(t, <-ch)
	}
}

func TestPutNextKeepsSequencesGrowing(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	seqs := []uint64{}
	put := func() {
		require.NoError(t, lit.PutNext(func(seq uint64) ([]byte, []byte, error) {
			seqs = append(seqs, seq)
			return []byte(fmt.Sprintf("/events/%016x", seq)), []byte("v"), nil
		}))
	}

	put()
	put()
	require.NoError(t, lit.Reopen())
	put()

	require.Len(t, seqs, 3)
	require.True(t, seqs[0] > 0)
	require.True(t, seqs[1] > seqs[0])
	require.True(t, seqs[2] > seqs[1])

	value, err := lit.Get([]byte(fmt.Sprintf("/events/%016x", seqs[2])))
	require.NoError(t, err)
	require.Equal(t, []byte("v"), value)
}

func TestPutNextFailedAsEntryError(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	require.Error(t, lit.PutNext(func(uint64) ([]byte, []byte, error) {
		return nil, nil, fmt.Errorf("entry error")
	}))
}

func TestScanOrderedByKeys(t *testing.T) {
	lit, cancel := newTestLithium(t)
	defer cancel()

	for i := 0xf; i > 0; i-- {
		key := []byte(fmt.Sprintf("/events/%016x", i))
		require.NoError(t, lit.Put(key, []byte("v")))
	}

	var last uint64
	ch, _ := lit.Scan([]byte("/events/"))
	for ent := range ch {
		require.NoError(t, ent.Error())

		key, _ := ent.Pair()
		raw := strings.TrimLeft(strings.TrimPrefix(string(key), "/events/"), "0")

		id, err := strconv.ParseUint(raw, 16, 64)
		require.NoError(t, err)
		require.True(t, id > last)

		last = id
	}
}

func newTestLithium(t *testing.T) (lit *Lithium, cancel func()) {
	path := filepath.Join(t.TempDir(), "lithium.wal")
	lit = NewLithium()
	require.NoError(t, lit.Open(path, 0o666, time.Second))

	cancel = func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		closed := make(chan struct{})
		go func() {
			defer close(closed)
			require.NoError(t, lit.Close())
		}()

		select {
		case <-ctx.Done():
			require.FailNow(t, "close error: %s", ctx.Err())
		case <-closed:
		}
	}

	return lit, cancel
}
