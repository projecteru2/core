package etcdv3

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func TestAddNode(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	nodename := "testnode"
	nodename2 := "testnode2"
	podname := "testpod"
	_, err := m.AddPod(ctx, podname, "test")
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, "numapod", "test")
	assert.NoError(t, err)
	labels := map[string]string{"test": "1"}

	endpoint := "mock://fakeengine"
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: "abc", Labels: labels})
	assert.Error(t, err)
	node, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, Labels: labels})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, nodename)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, Labels: labels})
	assert.Error(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: endpoint, Podname: podname, Labels: labels})
	assert.Error(t, err)
	key := fmt.Sprintf(common.NodeInfoKey, nodename)
	_, err = kvOf(m).GetOne(ctx, key)
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename2, Endpoint: endpoint, Podname: podname, Labels: labels})
	assert.NoError(t, err)
	nodename3 := "nodename3"
	endpoint3 := "tcp://path"
	node3, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename3, Endpoint: endpoint3, Podname: podname, Labels: labels})
	assert.NoError(t, err)
	_, err = m.MakeClient(ctx, node3)
	assert.Error(t, err)
}

func TestRemoveNode(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	_, err := m.AddPod(ctx, "testpod", "")
	assert.NoError(t, err)
	node, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod"})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	assert.NoError(t, m.RemoveNode(ctx, nil))
	assert.NoError(t, m.RemoveNode(ctx, node))
}

func TestGetNode(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	_, err := m.AddPod(ctx, "testpod", "")
	assert.NoError(t, err)
	node, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod"})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	_, err = m.GetNode(ctx, "wtf")
	assert.Error(t, err)
	n, err := m.GetNode(ctx, "test")
	assert.NoError(t, err)
	assert.Equal(t, node.Name, n.Name)
}

func TestGetNodesByPod(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	_, err := m.AddPod(ctx, "testpod", "")
	assert.NoError(t, err)
	node, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod"})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	ns, err := m.GetNodesByPod(ctx, &types.NodeFilter{Podname: "wtf", All: false}, false)
	assert.NoError(t, err)
	assert.Empty(t, ns)
	ns, err = m.GetNodesByPod(ctx, &types.NodeFilter{Podname: "testpod", All: true}, false)
	assert.NoError(t, err)
	assert.NotEmpty(t, ns)
	ns, err = m.GetNodesByPod(ctx, &types.NodeFilter{All: false}, false)
	assert.NoError(t, err)
	assert.Len(t, ns, 1)
	ns, err = m.GetNodesByPod(ctx, &types.NodeFilter{All: true}, false)
	assert.NoError(t, err)
	assert.NotEmpty(t, ns)
}

func TestUpdateNode(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	_, err := m.AddPod(ctx, "testpod", "")
	assert.NoError(t, err)
	node, err := m.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod"})
	assert.NoError(t, err)
	assert.Equal(t, node.Name, "test")
	fakeNode := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "nil",
			Podname:  "wtf",
			Endpoint: "mock://hh",
		},
	}
	assert.NoError(t, m.UpdateNodes(ctx, fakeNode))
	assert.NoError(t, m.UpdateNodes(ctx, node))
	node.Available = false
	assert.NoError(t, m.UpdateNodes(ctx, node))
}

func TestSetNodeStatus(t *testing.T) {
	assert := assert.New(t)
	m := NewMercury(t)
	node := newStatusNode(t, m)
	assert.NoError(m.SetNodeStatus(t.Context(), node, 1))
	key := filepath.Join(common.NodeStatusPrefix, node.Name)

	_, err := kvOf(m).GetOne(t.Context(), key)
	assert.NoError(err)
	time.Sleep(2000 * time.Millisecond)
	_, err = kvOf(m).GetOne(t.Context(), key)
	assert.Error(err)
}

func TestGetNodeStatus(t *testing.T) {
	assert := assert.New(t)
	m := NewMercury(t)
	node := newStatusNode(t, m)
	assert.NoError(m.SetNodeStatus(t.Context(), node, 1))

	ns, err := m.GetNodeStatus(t.Context(), node.Name)
	assert.NoError(err)
	assert.Equal(ns.Nodename, node.Name)
	assert.True(ns.Alive)
	time.Sleep(2 * time.Second)
	ns1, err := m.GetNodeStatus(t.Context(), node.Name)
	assert.Error(err)
	assert.Nil(ns1)
}

func TestNodeStatusStream(t *testing.T) {
	assert := assert.New(t)
	m := NewMercury(t)
	node := newStatusNode(t, m)

	go func() {
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(500 * time.Millisecond)
			assert.NoError(m.SetNodeStatus(t.Context(), node, 1))
		}
	}()

	ctx, cancel := context.WithCancel(t.Context())
	ch := m.NodeStatusStream(ctx)
	go func() {
		time.Sleep(3000 * time.Millisecond)
		cancel()
	}()

	statuses := []*types.NodeStatus{}
	for s := range ch {
		statuses = append(statuses, s)
	}
	for _, s := range statuses[:len(statuses)-1] {
		assert.True(s.Alive)
	}
	assert.False(statuses[len(statuses)-1].Alive)
}

func newStatusNode(t *testing.T, m *Mercury) *types.Node {
	assert := assert.New(t)
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "mock://",
			Podname:  "testpod",
		},
	}
	_, err := m.AddPod(t.Context(), node.Podname, "")
	assert.NoError(err)
	_, err = m.AddNode(t.Context(), &types.AddNodeOptions{
		Nodename: node.Name,
		Endpoint: node.Endpoint,
		Podname:  node.Podname,
	})
	assert.NoError(err)
	return node
}
