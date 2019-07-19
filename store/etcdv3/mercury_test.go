package etcdv3

import (
	"context"
	"testing"

	"github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/integration"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func InitCluster(t *testing.T) *integration.ClusterV3 {
	return integration.NewClusterV3(t, &integration.ClusterConfig{Size: 1})
}

func AfterTest(t *testing.T, cluster *integration.ClusterV3) {
	cluster.Terminate(t)
}

func NewMercury(t *testing.T, cliv3 *clientv3.Client) *Mercury {
	config := types.Config{}
	config.LockTimeout = 10
	config.Etcd = types.EtcdConfig{
		Machines:   []string{"127.0.0.1:2379"},
		Prefix:     "/eru-test",
		LockPrefix: "/eru-test-lock",
	}
	//	config.Docker.CertPath = "/tmp"

	return &Mercury{cliv3, config}
}

func TestMercury(t *testing.T) {
	etcd := InitCluster(t)
	defer AfterTest(t, etcd)
	m := NewMercury(t, etcd.RandClient())
	ctx := context.Background()

	// CreateLock
	_, err := m.CreateLock("test", 5)
	assert.NoError(t, err)
	// Get
	resp, err := m.Get(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, resp.Count, int64(0))
	// Put
	_, err = m.Put(ctx, "test/1", "a")
	m.Put(ctx, "test/2", "a")
	assert.NoError(t, err)
	// Get again
	resp, err = m.Get(ctx, "test/1")
	assert.NoError(t, err)
	assert.Equal(t, resp.Count, int64(len(resp.Kvs)))
	// GetOne
	_, err = m.GetOne(ctx, "test", clientv3.WithPrefix())
	assert.Error(t, err)
	ev, err := m.GetOne(ctx, "test/1")
	assert.NoError(t, err)
	assert.Equal(t, string(ev.Value), "a")
	// Delete
	_, err = m.Delete(ctx, "test/2")
	assert.NoError(t, err)
	m.Put(ctx, "d1", "a")
	m.Put(ctx, "d2", "a")
	m.Put(ctx, "d3", "a")
	// BatchDelete
	r, err := m.batchDelete(ctx, []string{"d1", "d2", "d3"})
	assert.NoError(t, err)
	assert.True(t, r.Succeeded)
	// Create
	r, err = m.Create(ctx, "test/2", "a")
	assert.NoError(t, err)
	assert.True(t, r.Succeeded)
	// CreateFail
	r, err = m.Create(ctx, "test/2", "a")
	assert.Error(t, err)
	assert.False(t, r.Succeeded)
	// BatchCreate
	data := map[string]string{
		"k1": "a1",
		"k2": "a2",
	}
	r, err = m.BatchCreate(ctx, data)
	assert.NoError(t, err)
	assert.True(t, r.Succeeded)
	// BatchCreateFailed
	r, err = m.BatchCreate(ctx, data)
	assert.Error(t, err)
	assert.False(t, r.Succeeded)
	// Update
	r, err = m.Update(ctx, "test/2", "b")
	assert.NoError(t, err)
	assert.True(t, r.Succeeded)
	// UpdateFail
	r, err = m.Update(ctx, "test/3", "b")
	assert.Error(t, err)
	assert.False(t, r.Succeeded)
	// BatchUpdate
	data = map[string]string{
		"k1": "b1",
		"k2": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	assert.NoError(t, err)
	assert.True(t, r.Succeeded)
	// BatchUpdateFail
	data = map[string]string{
		"k1": "c1",
		"k3": "b2",
	}
	r, err = m.BatchUpdate(ctx, data)
	assert.Error(t, err)
	assert.False(t, r.Succeeded)
	// Watch
	ctx2, cancel := context.WithCancel(ctx)
	ch := m.Watch(ctx2, "watchkey", clientv3.WithPrefix())
	go func() {
		for r := range ch {
			assert.NotEmpty(t, r.Events)
			assert.Equal(t, len(r.Events), 1)
			assert.Equal(t, r.Events[0].Type.String(), store.PutEvent)
			assert.Equal(t, string(r.Events[0].Kv.Value), "b")
		}
	}()
	m.Create(ctx, "watchkey/1", "b")
	cancel()
}
