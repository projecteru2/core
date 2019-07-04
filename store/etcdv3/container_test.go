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
	etcd := InitCluster(t)
	defer AfterTest(t, etcd)
	m := NewMercury(t, etcd.RandClient())
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
	ctx2, cancel := context.WithCancel(ctx)
	defer cancel()
	ch := m.WatchDeployStatus(ctx2, appname, entrypoint, "")
	go func() {
		for s := range ch {
			assert.Equal(t, s.Appname, appname)
		}
	}()
	assert.NoError(t, m.RemoveContainer(ctx, container))
	// bindContainerAdditions
	testC := &types.Container{
		Podname:  "t",
		Nodename: "x",
	}
	// failed by GetNode
	_, err = m.bindContainerAdditions(ctx, testC)
	assert.Error(t, err)
	testC.Nodename = nodename
	testC.Podname = podname
	// failed by ParseContainerName
	_, err = m.bindContainerAdditions(ctx, testC)
	assert.Error(t, err)
	// failed by GetOne
	testC.Name = name
	_, err = m.bindContainerAdditions(ctx, testC)
	assert.Error(t, err)
	// correct
	testC.ID = ID
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	_, err = m.bindContainerAdditions(ctx, testC)
	assert.NoError(t, err)
}
