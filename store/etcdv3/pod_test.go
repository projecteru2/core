package etcdv3

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestPod(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	podname := "testv3"

	pod, err := m.AddPod(ctx, podname, "CPU")
	assert.NoError(t, err)
	assert.Equal(t, pod.Name, podname)

	_, err = m.AddPod(ctx, podname, "CPU")
	assert.Equal(t, err, types.ErrKeyExists)

	pod2, err := m.GetPod(ctx, podname)
	assert.NoError(t, err)
	assert.Equal(t, pod2.Name, podname)

	pods, err := m.GetAllPods(ctx)
	assert.NoError(t, err)
	assert.Equal(t, len(pods), 1)
	assert.Equal(t, pods[0].Name, podname)

	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: podname})
	assert.NoError(t, err)
	err = m.RemovePod(ctx, podname)
	assert.Error(t, err)
	err = m.RemoveNode(ctx, &types.Node{NodeMeta: types.NodeMeta{Podname: podname, Name: "test", Endpoint: "mock://"}})
	assert.NoError(t, err)
	err = m.RemovePod(ctx, podname)
	assert.NoError(t, err)
}
