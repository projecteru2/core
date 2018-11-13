package etcdv3

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPod(t *testing.T) {
	etcd := InitCluster(t)
	defer AfterTest(t, etcd)
	m := NewMercury(t, etcd.RandClient())
	ctx := context.Background()
	podname := "testv3"

	pod, err := m.AddPod(ctx, podname, "CPU", "")
	assert.NoError(t, err)
	assert.Equal(t, pod.Name, podname)

	pod2, err := m.GetPod(ctx, podname)
	assert.NoError(t, err)
	assert.Equal(t, pod2.Name, podname)

	pods, err := m.GetAllPods(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name, podname)

	err = m.RemovePod(ctx, podname)
	assert.NoError(t, err)
}
