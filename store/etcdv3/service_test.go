package etcdv3

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServiceStatusStream(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx, cancel := context.WithCancel(context.Background())
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
