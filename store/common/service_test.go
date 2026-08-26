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

func TestServiceStatusStreamRecoversAfterSnapshotFailure(t *testing.T) {
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	defer pool.Release()
	ctx, cancel := context.WithCancel(context.Background())
	store := New(&recoveringServiceKV{}, types.Config{ConnectionTimeout: time.Millisecond}, pool)

	ch, err := store.ServiceStatusStream(ctx)
	require.NoError(t, err)
	select {
	case endpoints, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, []string{"127.0.0.1:5001"}, endpoints)
	case <-time.After(time.Second):
		t.Fatal("service status stream did not recover")
	}
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("service status stream did not stop")
	}
}

type recoveringServiceKV struct {
	KV

	reads atomic.Int32
}

func (k *recoveringServiceKV) GetPrefix(context.Context, string, int64) (map[string]string, error) {
	if k.reads.Add(1) == 1 {
		return nil, types.ErrMockError
	}
	return map[string]string{"/services/127.0.0.1:5001": ""}, nil
}

func (k *recoveringServiceKV) Watch(ctx context.Context, _ string) iter.Seq[Event] {
	return func(func(Event) bool) {
		<-ctx.Done()
	}
}
