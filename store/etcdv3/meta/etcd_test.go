package meta

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/require"
	"go.etcd.io/etcd/clientv3"
)

func NewEmbeddedETCD(t *testing.T) *ETCD {
	config := types.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	e, err := NewETCD(config, true)
	require.NoError(t, err)
	return e
}

func TestETCD(t *testing.T) {
	m := NewEmbeddedETCD(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()

	// CreateLock
	_, err := m.CreateLock("test", 5)
	require.NoError(t, err)
	// Get
	resp, err := m.Get(ctx, "test")
	require.NoError(t, err)
	require.Equal(t, resp.Count, int64(0))
	// Put
	_, err = m.Put(ctx, "test/1", "a")
	m.Put(ctx, "test/2", "a")
	require.NoError(t, err)
	// Get again
	resp, err = m.Get(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, resp.Count, int64(len(resp.Kvs)))
	// GetOne
	_, err = m.GetOne(ctx, "test", clientv3.WithPrefix())
	require.Error(t, err)
	ev, err := m.GetOne(ctx, "test/1")
	require.NoError(t, err)
	require.Equal(t, string(ev.Value), "a")
	// Delete
	_, err = m.Delete(ctx, "test/2")
	require.NoError(t, err)
	m.Put(ctx, "d1", "a")
	m.Put(ctx, "d2", "a")
	m.Put(ctx, "d3", "a")
	// BatchDelete
	r, err := m.BatchDelete(ctx, []string{"d1", "d2", "d3"})
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// Create
	r, err = m.Create(ctx, "test/2", "a")
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// CreateFail
	r, err = m.Create(ctx, "test/2", "a")
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// BatchCreate
	data := map[string]string{
		"k1": "a1",
		"k2": "a2",
	}
	r, err = m.BatchCreate(ctx, data)
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// BatchCreateFailed
	r, err = m.BatchCreate(ctx, data)
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// Update
	r, err = m.Update(ctx, "test/2", "b")
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// UpdateFail
	r, err = m.Update(ctx, "test/3", "b")
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// BatchUpdate
	data = map[string]string{
		"k1": "b1",
		"k2": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	require.NoError(t, err)
	require.True(t, r.Succeeded)
	// BatchUpdateFail
	data = map[string]string{
		"k1": "c1",
		"k3": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	require.Error(t, err)
	require.False(t, r.Succeeded)
	// Watch
	ctx2, cancel := context.WithCancel(ctx)
	ch := m.watch(ctx2, "watchkey", clientv3.WithPrefix())
	go func() {
		for r := range ch {
			require.NotEmpty(t, r.Events)
			require.Equal(t, len(r.Events), 1)
			require.Equal(t, r.Events[0].Type, clientv3.EventTypePut)
			require.Equal(t, string(r.Events[0].Kv.Value), "b")
		}
	}()
	m.Create(ctx, "watchkey/1", "b")
	cancel()
}
