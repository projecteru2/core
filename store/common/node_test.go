package common

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestNodeStatusStreamClosesWhenWatchBreaks(t *testing.T) {
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	defer pool.Release()
	store := New(&brokenWatchKV{key: "node1"}, types.Config{ConnectionTimeout: time.Millisecond}, pool)

	ch := store.NodeStatusStream(t.Context())
	select {
	case status, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, "node1", status.Nodename)
		assert.True(t, status.Alive)
	case <-time.After(time.Second):
		t.Fatal("node status stream delivered nothing")
	}
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("node status stream did not close after the watch broke")
	}
}

type brokenWatchKV struct {
	KV

	key string
}

func (k *brokenWatchKV) GetOne(context.Context, string) (string, error) {
	return "", types.ErrMockError
}

func (k *brokenWatchKV) GetMulti(context.Context, []string) (map[string]string, error) {
	return nil, types.ErrMockError
}

func (k *brokenWatchKV) Watch(_ context.Context, prefix string) iter.Seq[Event] {
	return func(yield func(Event) bool) {
		yield(Event{Key: prefix + k.key, Type: EventPut})
	}
}

func (k *brokenWatchKV) NotFound(error) bool { return false }
