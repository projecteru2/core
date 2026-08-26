package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func TestWorkloadStatusStreamClosesWhenWatchBreaks(t *testing.T) {
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	defer pool.Release()
	store := New(&brokenWatchKV{key: "wid1"}, types.Config{ConnectionTimeout: time.Millisecond}, pool)

	ch := store.WorkloadStatusStream(t.Context(), "app", "entry", "node1", nil)
	select {
	case msg, ok := <-ch:
		require.True(t, ok)
		assert.Equal(t, "wid1", msg.ID)
		assert.Error(t, msg.Error)
	case <-time.After(time.Second):
		t.Fatal("workload status stream delivered nothing")
	}
	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(time.Second):
		t.Fatal("workload status stream did not close after the watch broke")
	}
}
