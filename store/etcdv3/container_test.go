package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestContainer(t *testing.T) {
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
	// Add
	assert.NoError(t, m.AddContainer(ctx, container))
	_, err = m.GetOne(ctx, fmt.Sprintf(containerInfoKey, container.ID))
	assert.NoError(t, err)
	_, err = m.GetOne(ctx, fmt.Sprintf(nodeContainersKey, container.Nodename, container.ID))
	assert.NoError(t, err)
	r, err := m.GetOne(ctx, filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID))
	assert.NoError(t, err)
	assert.NotEmpty(t, r.Value)
	// Update
	container.Memory = int64(100)
	container.Storage = int64(100)
	assert.NoError(t, m.UpdateContainer(ctx, container))
	// RemoveFail
	container.ID = "a"
	assert.Error(t, m.RemoveContainer(ctx, container))
	// GetFail
	_, err = m.GetContainer(ctx, "a")
	assert.Error(t, err)
	// GetContainers
	container.ID = ID
	containers, err := m.GetContainers(ctx, []string{ID})
	assert.NoError(t, err)
	assert.Equal(t, len(containers), 1)
	assert.Equal(t, containers[0].Name, name)
	// GetContainers for multiple containers
	newContainer := &types.Container{
		ID:       "1234567812345678123456781234567812345678123456781234567812340000",
		Name:     "test_app_2",
		Nodename: nodename,
		Podname:  podname,
	}
	assert.NoError(t, m.AddContainer(ctx, newContainer))
	containers, err = m.GetContainers(ctx, []string{container.ID, newContainer.ID})
	assert.NoError(t, err)
	assert.Equal(t, len(containers), 2)
	assert.Equal(t, containers[0].Name, container.Name)
	assert.Equal(t, containers[1].Name, newContainer.Name)
	assert.NoError(t, m.RemoveContainer(ctx, newContainer))
	// Deployed
	container.StatusMeta = &types.StatusMeta{
		Running: true,
	}
	err = m.SetContainerStatus(ctx, container, 0)
	assert.NoError(t, err)
	container2 := &types.Container{
		ID:         container.ID,
		Nodename:   "n2",
		StatusMeta: &types.StatusMeta{Healthy: true},
	}
	err = m.SetContainerStatus(ctx, container2, 0)
	assert.Error(t, err)
	// ListContainers
	containers, _ = m.ListContainers(ctx, appname, entrypoint, "", 1)
	assert.Equal(t, len(containers), 1)
	assert.Equal(t, containers[0].Name, name)
	// ListNodeContainers
	containers, _ = m.ListNodeContainers(ctx, nodename)
	assert.Equal(t, len(containers), 1)
	assert.Equal(t, containers[0].Name, name)
	containers, _ = m.ListNodeContainers(ctx, "n2")
	assert.Equal(t, len(containers), 0)
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
	cctx, cancel := context.WithCancel(ctx)
	ch := m.ContainerStatusStream(cctx, appname, entrypoint, "", nil)
	container.StatusMeta = &types.StatusMeta{
		ID:      ID,
		Running: true,
	}
	assert.NoError(t, m.SetContainerStatus(ctx, container, 0))
	s := <-ch
	assert.False(t, s.Delete)
	assert.NotNil(t, s.Container)
	cancel()
}
