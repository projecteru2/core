package common

import (
	"context"
	"iter"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestStatusStreamsDoNotOccupyPool(t *testing.T) {
	tests := []struct {
		name  string
		start func(context.Context, *Store) error
	}{
		{"service", func(ctx context.Context, store *Store) error {
			store.ServiceStatusStream(ctx)
			return nil
		}},
		{"node", func(ctx context.Context, store *Store) error {
			store.NodeStatusStream(ctx)
			return nil
		}},
		{"workload", func(ctx context.Context, store *Store) error {
			store.WorkloadStatusStream(ctx, "", "", "", nil)
			return nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				pool, err := utils.NewPool(1)
				require.NoError(t, err)
				defer pool.Release()
				ctx, cancel := context.WithCancel(t.Context())
				store := New(&blockingServiceKV{}, types.Config{}, pool)

				require.NoError(t, tt.start(ctx, store))
				ran := make(chan struct{})
				invokeDone := make(chan error, 1)
				go func() {
					invokeDone <- pool.Invoke(func() { close(ran) })
				}()
				synctest.Wait()
				select {
				case <-ran:
				default:
					t.Errorf("%s status stream occupied the pool", tt.name)
				}

				cancel()
				synctest.Wait()
				require.NoError(t, <-invokeDone)
			})
		})
	}
}

func TestServiceStatusStreamRecoversAfterSnapshotFailure(t *testing.T) {
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	defer pool.Release()
	ctx, cancel := context.WithCancel(t.Context())
	store := New(&recoveringServiceKV{}, types.Config{ConnectionTimeout: time.Millisecond}, pool)

	ch := store.ServiceStatusStream(ctx)
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

func (k *recoveringServiceKV) NotFound(error) bool { return false }

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

type blockingServiceKV struct {
	KV
}

func (k *blockingServiceKV) NotFound(error) bool { return false }

func (k *blockingServiceKV) GetPrefix(context.Context, string, int64) (map[string]string, error) {
	return map[string]string{}, nil
}

func (k *blockingServiceKV) Watch(ctx context.Context, _ string) iter.Seq[Event] {
	return func(func(Event) bool) {
		<-ctx.Done()
	}
}
