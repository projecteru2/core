package common

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
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

func TestGetWorkloadsReadsOneStatusPrefixPerGroup(t *testing.T) {
	kv := newStatusPrefixKV(t)
	store := newStatusPrefixStore(t, kv)

	workloads, err := store.getWorkloads(t.Context(), []string{"w1", "w2", "w3", "w4"}, false)
	require.NoError(t, err)
	require.Len(t, workloads, 4)

	assert.Equal(t, []string{"/status/test/app/n1/", "/status/test/app/n2/", "/status/test/web/n1/"}, kv.readPrefixes())
	require.NotNil(t, workloads[0].StatusMeta)
	assert.True(t, workloads[0].StatusMeta.Running)
	assert.Nil(t, workloads[1].StatusMeta, "a workload without a status key keeps a nil StatusMeta")
	require.NotNil(t, workloads[2].StatusMeta)
	assert.Equal(t, "w3", workloads[2].StatusMeta.ID)
	assert.Nil(t, workloads[3].StatusMeta, "an unmarshalable status is skipped")
}

func TestGetWorkloadReadsOneStatusPrefix(t *testing.T) {
	kv := newStatusPrefixKV(t)
	store := newStatusPrefixStore(t, kv)

	workload, err := store.getWorkload(t.Context(), "w1", false)
	require.NoError(t, err)
	assert.Equal(t, []string{"/status/test/app/n1/"}, kv.readPrefixes())
	require.NotNil(t, workload.StatusMeta)
	assert.Equal(t, "w1", workload.StatusMeta.ID)
}

func TestGetWorkloadsFailsWhenStatusReadFails(t *testing.T) {
	kv := newStatusPrefixKV(t)
	kv.err = types.ErrMockError
	store := newStatusPrefixStore(t, kv)

	_, err := store.getWorkloads(t.Context(), []string{"w1", "w3"}, false)
	assert.ErrorIs(t, err, types.ErrMockError)
}

func newStatusPrefixStore(t *testing.T, kv KV) *Store {
	t.Helper()
	pool, err := utils.NewPool(1)
	require.NoError(t, err)
	t.Cleanup(pool.Release)
	return New(kv, types.Config{}, pool)
}

func newStatusPrefixKV(t *testing.T) *statusPrefixKV {
	t.Helper()
	kv := &statusPrefixKV{workloads: map[string]string{}, statuses: map[string]string{}}
	for _, workload := range []*types.Workload{
		{ID: "w1", Name: "test_app_1", Nodename: "n1"},
		{ID: "w2", Name: "test_app_2", Nodename: "n1"},
		{ID: "w3", Name: "test_app_3", Nodename: "n2"},
		{ID: "w4", Name: "test_web_1", Nodename: "n1"},
	} {
		data, err := json.Marshal(workload)
		require.NoError(t, err)
		kv.workloads[fmt.Sprintf(WorkloadInfoKey, workload.ID)] = string(data)
	}
	for key, ID := range map[string]string{
		"/status/test/app/n1/w1": "w1",
		"/status/test/app/n1/w9": "w9",
		"/status/test/app/n2/w3": "w3",
	} {
		data, err := json.Marshal(&types.StatusMeta{ID: ID, Running: true})
		require.NoError(t, err)
		kv.statuses[key] = string(data)
	}
	kv.statuses["/status/test/web/n1/w4"] = "}"
	return kv
}

type statusPrefixKV struct {
	KV

	workloads map[string]string
	statuses  map[string]string
	err       error

	mu       sync.Mutex
	prefixes []string
}

func (k *statusPrefixKV) GetMulti(_ context.Context, keys []string) (map[string]string, error) {
	data := make(map[string]string, len(keys))
	for _, key := range keys {
		data[key] = k.workloads[key]
	}
	return data, nil
}

func (k *statusPrefixKV) GetPrefix(_ context.Context, prefix string, _ int64) (map[string]string, error) {
	k.mu.Lock()
	k.prefixes = append(k.prefixes, prefix)
	k.mu.Unlock()
	if k.err != nil {
		return nil, k.err
	}
	data := map[string]string{}
	for key, value := range k.statuses {
		if strings.HasPrefix(key, prefix) {
			data[key] = value
		}
	}
	return data, nil
}

func (k *statusPrefixKV) readPrefixes() []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	return slices.Sorted(slices.Values(k.prefixes))
}
