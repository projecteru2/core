package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestAddORUpdateContainer(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     "a",
	}
	// failed by name
	err := m.AddContainer(ctx, container)
	assert.Error(t, err)
	container.Name = name
	// fail update
	err = m.UpdateContainer(ctx, container)
	assert.Error(t, err)
	// success create
	err = m.AddContainer(ctx, container)
	assert.NoError(t, err)
	// success updat
	err = m.UpdateContainer(ctx, container)
	assert.NoError(t, err)
}

func TestRemoveContainer(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := m.AddContainer(ctx, container)
	assert.NoError(t, err)
	// fail remove
	container.Name = "a"
	err = m.RemoveContainer(ctx, container)
	assert.Error(t, err)
	container.Name = name
	// success remove
	err = m.RemoveContainer(ctx, container)
	assert.NoError(t, err)
}

func TestGetContainer(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := m.AddContainer(ctx, container)
	assert.NoError(t, err)
	// failed by no container
	_, err = m.GetContainers(ctx, []string{ID, "xxx"})
	assert.Error(t, err)
	// failed by no pod nodes
	_, err = m.GetContainer(ctx, ID)
	assert.Error(t, err)
	// create pod node
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, nodename, "mock://", podname, "", "", "", 10, 100, 1000, 1000, nil, nil, nil)
	assert.NoError(t, err)
	// success
	_, err = m.GetContainer(ctx, ID)
	assert.NoError(t, err)
}

func TestGetContainerStatus(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := m.AddContainer(ctx, container)
	assert.NoError(t, err)
	// failed no pod no node
	_, err = m.GetContainerStatus(ctx, ID)
	assert.Error(t, err)
	// add success
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, nodename, "mock://", podname, "", "", "", 10, 100, 1000, 1000, nil, nil, nil)
	assert.NoError(t, err)
	c, err := m.GetContainerStatus(ctx, ID)
	assert.Nil(t, c)
}

func TestSetContainerStatus(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
	}
	// fail by no name
	err := m.SetContainerStatus(ctx, container, 0)
	assert.Error(t, err)
	container.Name = name
	// success
	err = m.SetContainerStatus(ctx, container, 10)
	assert.NoError(t, err)
}

func TestListContainers(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	// no key
	cs, err := m.ListContainers(ctx, "", "a", "b", 1, nil)
	assert.NoError(t, err)
	assert.Empty(t, cs)
	// add container
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
		Labels:   map[string]string{"x": "y"},
	}
	// success create
	err = m.AddContainer(ctx, container)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, nodename, "mock://", podname, "", "", "", 10, 100, 1000, 1000, nil, nil, nil)
	assert.NoError(t, err)
	// no labels
	cs, err = m.ListContainers(ctx, "", "a", "b", 1, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, cs)
	// labels
	cs, err = m.ListContainers(ctx, "", "a", "b", 1, map[string]string{"x": "z"})
	assert.NoError(t, err)
	assert.Empty(t, cs)
}

func TestListNodeContainers(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	// no key
	cs, err := m.ListNodeContainers(ctx, "", nil)
	assert.NoError(t, err)
	assert.Empty(t, cs)
	// add container
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	container := &types.Container{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
		Labels:   map[string]string{"x": "y"},
	}
	// success create
	err = m.AddContainer(ctx, container)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, nodename, "mock://", podname, "", "", "", 10, 100, 1000, 1000, nil, nil, nil)
	assert.NoError(t, err)
	// no labels
	cs, err = m.ListNodeContainers(ctx, nodename, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, cs)
	// labels
	cs, err = m.ListNodeContainers(ctx, nodename, map[string]string{"x": "z"})
	assert.NoError(t, err)
	assert.Empty(t, cs)
}

func TestContainerStatusStream(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	appname := "test"
	entrypoint := "app"
	nodename := "n1"
	podname := "test"
	container := &types.Container{
		ID:       ID,
		Name:     name,
		Nodename: nodename,
		Podname:  podname,
	}
	node := &types.Node{
		Name:     nodename,
		Podname:  podname,
		Endpoint: "tcp://127.0.0.1:2376",
	}
	_, err := json.Marshal(container)
	assert.NoError(t, err)
	nodeBytes, err := json.Marshal(node)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, podname, "CPU")
	assert.NoError(t, err)
	_, err = m.Create(ctx, fmt.Sprintf(nodeInfoKey, podname, nodename), string(nodeBytes))
	assert.NoError(t, err)
	_, err = m.Create(ctx, fmt.Sprintf(nodePodKey, nodename), podname)
	assert.NoError(t, err)
	assert.NoError(t, m.AddContainer(ctx, container))
	// ContainerStatusStream
	container.StatusMeta = &types.StatusMeta{
		ID:      ID,
		Running: true,
	}
	cctx, cancel := context.WithCancel(ctx)
	ch := m.ContainerStatusStream(cctx, appname, entrypoint, "", nil)
	assert.NoError(t, m.SetContainerStatus(ctx, container, 0))
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()
	for s := range ch {
		assert.False(t, s.Delete)
		assert.NotNil(t, s.Container)
	}
}
