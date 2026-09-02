package etcdv3

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/store/common"
)

func TestRegisterServiceWithDeregister(t *testing.T) {
	m := NewMercury(t)

	ctx := t.Context()
	svc := "svc"
	path := fmt.Sprintf(common.ServiceStatusKey, svc)
	_, deregister, err := m.RegisterService(ctx, svc, time.Minute)
	assert.NoError(t, err)

	kv, err := kvOf(m).GetOne(ctx, path)
	assert.NoError(t, err)
	assert.Equal(t, path, string(kv.Key))

	deregister()
	kv, err = kvOf(m).GetOne(ctx, path)
	assert.Error(t, err)
	assert.Nil(t, kv)
}

func TestServiceStatusStream(t *testing.T) {
	m := NewMercury(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, unregisterService1, err := m.RegisterService(ctx, "127.0.0.1:5001", time.Second)
	assert.NoError(t, err)
	ch, err := m.ServiceStatusStream(ctx)
	assert.NoError(t, err)
	assert.Equal(t, <-ch, []string{"127.0.0.1:5001"})
	_, _, err = m.RegisterService(ctx, "127.0.0.1:5002", time.Second)
	assert.NoError(t, err)
	endpoints := <-ch
	sort.Strings(endpoints)
	assert.Equal(t, endpoints, []string{"127.0.0.1:5001", "127.0.0.1:5002"})
	unregisterService1()
	assert.Equal(t, <-ch, []string{"127.0.0.1:5002"})
}
