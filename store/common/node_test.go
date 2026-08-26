package common

import (
	"context"
	"iter"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestNodeStatusStreamRecoversAfterWatchFailure(t *testing.T) {
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	defer pool.Release()
	ctx, cancel := context.WithCancel(context.Background())
	store := New(&recoveringWatchKV{key: "node1"}, types.Config{ConnectionTimeout: time.Millisecond}, pool)

	ch := store.NodeStatusStream(ctx)
	select {
	case status, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, "node1", status.Nodename)
		assert.True(t, status.Alive)
	case <-time.After(time.Second):
		t.Fatal("node status stream did not recover")
	}
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("node status stream did not stop")
	}
}

type recoveringWatchKV struct {
	KV

	watches atomic.Int32
	key     string
}

func (k *recoveringWatchKV) GetOne(context.Context, string) (string, error) {
	return "", types.ErrMockError
}

func (k *recoveringWatchKV) GetMulti(context.Context, []string) (map[string]string, error) {
	return nil, types.ErrMockError
}

func (k *recoveringWatchKV) Watch(ctx context.Context, prefix string) iter.Seq[Event] {
	return func(yield func(Event) bool) {
		if k.watches.Add(1) == 1 {
			return
		}
		if !yield(Event{Key: prefix + k.key, Type: EventPut}) {
			return
		}
		<-ctx.Done()
	}
}
