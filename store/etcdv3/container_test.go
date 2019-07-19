package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
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
		Name:     name,
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
	}
	node := &types.Node{
		Name:     nodename,
		Podname:  podname,
		Endpoint: "tcp://127.0.0.1:2376",
	}
	bytes, err := json.Marshal(container)
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
	r, err := m.GetOne(ctx, fmt.Sprintf(containerInfoKey, container.ID))
	assert.NoError(t, err)
	assert.Equal(t, r.Value, bytes)
	r, err = m.GetOne(ctx, fmt.Sprintf(nodeContainersKey, container.Nodename, container.ID))
	assert.NoError(t, err)
	assert.Equal(t, r.Value, bytes)
	r, err = m.GetOne(ctx, filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID))
	assert.NoError(t, err)
	assert.Equal(t, string(r.Value), "")
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
		Name:     "test_app_2",
		ID:       "1234567812345678123456781234567812345678123456781234567812340000",
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
	assert.NoError(t, m.ContainerDeployed(ctx, ID, appname, entrypoint, nodename, []byte{}, 0))
	assert.NoError(t, m.ContainerDeployed(ctx, ID, appname, entrypoint, nodename, []byte{}, 10))
	assert.Error(t, m.ContainerDeployed(ctx, ID, appname, entrypoint, "n2", []byte(""), 0))
	// ListContainers
	containers, _ = m.ListContainers(ctx, appname, entrypoint, "")
	assert.Equal(t, len(containers), 1)
	assert.Equal(t, containers[0].Name, name)
	// ListNodeContainers
	containers, _ = m.ListNodeContainers(ctx, nodename)
	assert.Equal(t, len(containers), 1)
	assert.Equal(t, containers[0].Name, name)
	containers, _ = m.ListNodeContainers(ctx, "n2")
	assert.Equal(t, len(containers), 0)
	// WatchDeployStatus
	ctx2 := context.Background()
	ch := m.WatchDeployStatus(ctx2, appname, entrypoint, "")
	assert.NoError(t, m.ContainerDeployed(ctx2, ID, appname, entrypoint, nodename, []byte("something"), 0))
	done := make(chan int)
	go func() {
		s := <-ch
		assert.Equal(t, s.Appname, appname)
		done <- 1
	}()
	assert.NoError(t, m.RemoveContainer(ctx, container))
	<-done
}
