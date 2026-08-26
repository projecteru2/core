package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestWorkloadStatusStreamRecoversAfterWatchFailure(t *testing.T) {
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	defer pool.Release()
	ctx, cancel := context.WithCancel(context.Background())
	store := New(&recoveringWatchKV{key: "wid1"}, types.Config{ConnectionTimeout: time.Millisecond}, pool)

	ch := store.WorkloadStatusStream(ctx, "app", "entry", "node1", nil)
	select {
	case msg, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, "wid1", msg.ID)
		assert.Error(t, msg.Error)
	case <-time.After(time.Second):
		t.Fatal("workload status stream did not recover")
	}
	cancel()
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("workload status stream did not stop")
	}
}
