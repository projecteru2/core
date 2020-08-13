package etcdv3

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegisterService(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	expire := 100 * time.Millisecond
	err := m.RegisterService(ctx, "127.0.0.1:5002", expire)
	assert.NoError(t, err)
	kv, err := m.GetOne(ctx, "/services/127.0.0.1:5002")
	assert.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(kv.Key), "127.0.0.1:5002"))
}

func TestUnregisterService(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	addr := "127.0.0.1:5002"
	assert.NoError(t, m.RegisterService(ctx, addr, time.Second))
	assert.NoError(t, m.UnregisterService(ctx, addr))
	_, err := m.GetOne(ctx, "/services/127.0.0.1:5002")
	assert.Error(t, err, "bad `Count` value")
}

func TestServiceStatusStream(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	assert.NoError(t, m.RegisterService(ctx, "127.0.0.1:5001", time.Second))
	ch, err := m.ServiceStatusStream(ctx)
	assert.NoError(t, err)
	assert.Equal(t, <-ch, []string{"127.0.0.1:5001"})
	assert.NoError(t, m.RegisterService(ctx, "127.0.0.1:5002", time.Second))
	endpoints := <-ch
	sort.Strings(endpoints)
	assert.Equal(t, endpoints, []string{"127.0.0.1:5001", "127.0.0.1:5002"})
	assert.NoError(t, m.UnregisterService(ctx, "127.0.0.1:5001"))
	assert.Equal(t, <-ch, []string{"127.0.0.1:5002"})
}
