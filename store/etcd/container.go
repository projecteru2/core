package etcdstore

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"context"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// get a container
// container if must be in full length, or we can't find it in etcd
// storage path in etcd is `/container/:containerid`
func (k *krypton) GetContainer(id string) (*types.Container, error) {
	if len(id) != 64 {
		return nil, fmt.Errorf("Container ID must be length of 64")
	}

	key := fmt.Sprintf(containerInfoKey, id)
	resp, err := k.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, fmt.Errorf("Container storage path %q in etcd is a directory", key)
	}

	c := &types.Container{}
	if err := json.Unmarshal([]byte(resp.Node.Value), c); err != nil {
		return nil, err
	}

	node, err := k.GetNode(c.Podname, c.Nodename)
	if err != nil {
		return nil, err
	}

	c.Engine = node.Engine
	return c, nil
}

// get many containers
func (k *krypton) GetContainers(ids []string) (containers []*types.Container, err error) {
	for _, id := range ids {
		container, err := k.GetContainer(id)
		if err != nil {
			return containers, err
		}
		containers = append(containers, container)
	}
	return containers, err
}

// add a container
// mainly record its relationship on pod and node
// actually if we already know its node, we will know its pod
// but we still store it
// storage path in etcd is `/container/:containerid`
func (k *krypton) AddContainer(container *types.Container) error {
	// now everything is ok
	// we use full length id instead
	key := fmt.Sprintf(containerInfoKey, container.ID)
	bytes, err := json.Marshal(container)
	if err != nil {
		return err
	}

	//add deploy status by agent now
	_, err = k.etcd.Set(context.Background(), key, string(bytes), nil)
	if err != nil {
		return err
	}

	// store deploy status
	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}

	key = fmt.Sprintf(containerDeployKey, appname, entrypoint, container.Nodename, container.ID)
	_, err = k.etcd.Set(context.Background(), key, "", nil)
	return err
}

func (k *krypton) ContainerDeployed(ID, appname, entrypoint, nodename, data string) error {
	key := fmt.Sprintf(containerDeployKey, appname, entrypoint, nodename, ID)
	//Only update when it exist
	_, err := k.etcd.Update(context.Background(), key, data)
	return err
}

// remove a container
// container id must be in full length
func (k *krypton) RemoveContainer(container *types.Container) error {
	if len(container.ID) < 64 {
		return fmt.Errorf("Container ID must be length of 64")
	}
	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}

	return k.CleanContainerData(container.ID, appname, entrypoint, container.Nodename)
}

func (k *krypton) CleanContainerData(ID, appname, entrypoint, nodename string) error {
	key := fmt.Sprintf(containerInfoKey, ID)
	if _, err := k.etcd.Delete(context.Background(), key, nil); err != nil {
		return err
	}

	// remove deploy status by core
	key = fmt.Sprintf(containerDeployKey, appname, entrypoint, nodename, ID)
	_, err := k.etcd.Delete(context.Background(), key, nil)
	return err
}

func (k *krypton) WatchDeployStatus(appname, entrypoint, nodename string) etcdclient.Watcher {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename)
	return k.etcd.Watcher(key, &etcdclient.WatcherOptions{Recursive: true})
}

func (k *krypton) ListContainers(appname, entrypoint, nodename string) ([]*types.Container, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename)
	resp, err := k.etcd.Get(context.Background(), key, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		return []*types.Container{}, err
	}
	containerIDs := getContainerDeployData(resp.Node.Key, resp.Node.Nodes)
	return k.GetContainers(containerIDs)
}

func getContainerDeployData(prefix string, nodes etcdclient.Nodes) []string {
	result := []string{}
	for _, node := range nodes {
		if len(node.Nodes) > 0 {
			result = append(result, getContainerDeployData(node.Key, node.Nodes)...)
		} else {
			key := strings.TrimLeft(strings.TrimLeft(node.Key, prefix), "/")
			result = append(result, key)
		}
	}
	return result
}
