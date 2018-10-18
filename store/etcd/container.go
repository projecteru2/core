package etcdstore

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"context"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// AddContainer add a container
// mainly record its relationship on pod and node
// actually if we already know its node, we will know its pod
// but we still store it
// storage path in etcd is `/container/:containerid`
func (k *Krypton) AddContainer(ctx context.Context, container *types.Container) error {
	var err error

	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			k.CleanContainerData(context.Background(), container.ID, appname, entrypoint, container.Nodename)
		}
	}()

	// now everything is ok
	// we use full length id instead
	key := fmt.Sprintf(containerInfoKey, container.ID)
	bytes, err := json.Marshal(container)
	if err != nil {
		return err
	}
	data := string(bytes)

	//add deploy status by agent now
	_, err = k.etcd.Set(ctx, key, data, nil)
	if err != nil {
		return err
	}

	// store deploy status
	key = filepath.Join(containerDeployPrefix, appname, entrypoint, container.Nodename, container.ID)
	_, err = k.etcd.Set(ctx, key, "", nil)
	if err != nil {
		return err
	}

	// store node-container data
	key = fmt.Sprintf(nodeContainersKey, container.Nodename, container.ID)
	_, err = k.etcd.Set(ctx, key, data, nil)

	return err
}

// RemoveContainer remove a container
// container id must be in full length
func (k *Krypton) RemoveContainer(ctx context.Context, container *types.Container) error {
	if l := len(container.ID); l != 64 {
		return types.NewDetailedErr(types.ErrBadContainerID,
			fmt.Sprintf("containerID: %s, length: %d",
				container.ID, len(container.ID)))
	}
	appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
	if err != nil {
		return err
	}

	return k.CleanContainerData(ctx, container.ID, appname, entrypoint, container.Nodename)
}

// CleanContainerData clean container data
func (k *Krypton) CleanContainerData(ctx context.Context, ID, appname, entrypoint, nodename string) error {
	key := fmt.Sprintf(containerInfoKey, ID)
	if _, err := k.etcd.Delete(ctx, key, nil); err != nil {
		return err
	}

	// remove deploy status by core
	key = filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	if _, err := k.etcd.Delete(ctx, key, nil); err != nil {
		return err
	}

	// remove node-containers data
	key = fmt.Sprintf(nodeContainersKey, nodename, ID)
	_, err := k.etcd.Delete(ctx, key, nil)
	return err

}

// GetContainer get a container
// container if must be in full length, or we can't find it in etcd
// storage path in etcd is `/container/:containerid`
func (k *Krypton) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	if l := len(ID); l != 64 {
		return nil, types.NewDetailedErr(types.ErrBadContainerID,
			fmt.Sprintf("containerID: %s, length: %d", ID, l))
	}

	key := fmt.Sprintf(containerInfoKey, ID)
	resp, err := k.etcd.Get(ctx, key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, types.NewDetailedErr(types.ErrKeyIsDir,
			fmt.Sprintf("container storage path %q in etcd is a directory", key))
	}

	c := &types.Container{}
	if err := json.Unmarshal([]byte(resp.Node.Value), c); err != nil {
		return nil, err
	}

	if c, err = k.bindNodeEngine(ctx, c); err != nil {
		return nil, err
	}

	return c, nil
}

// GetContainers get many containers
func (k *Krypton) GetContainers(ctx context.Context, IDs []string) (containers []*types.Container, err error) {
	for _, ID := range IDs {
		container, err := k.GetContainer(ctx, ID)
		if err != nil {
			return containers, err
		}
		containers = append(containers, container)
	}
	return containers, err
}

// ContainerDeployed store deployed container info
func (k *Krypton) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error {
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename, ID)
	//Only update when it exist
	_, err := k.etcd.Update(ctx, key, data)
	return err
}

// ListContainers list containers
func (k *Krypton) ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename)
	resp, err := k.etcd.Get(ctx, key, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		return []*types.Container{}, err
	}
	containerIDs := getContainerDeployData(resp.Node.Key, resp.Node.Nodes)
	return k.GetContainers(ctx, containerIDs)
}

// ListNodeContainers list containers belong to one node
func (k *Krypton) ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error) {
	key := fmt.Sprintf(nodeContainers, nodename)
	resp, err := k.etcd.Get(ctx, key, &etcdclient.GetOptions{Recursive: true})
	if err != nil {
		return []*types.Container{}, err
	}
	containers := []*types.Container{}
	for _, n := range resp.Node.Nodes {
		c := &types.Container{}
		if err := json.Unmarshal([]byte(n.Value), c); err != nil {
			return []*types.Container{}, err
		}

		if c, err = k.bindNodeEngine(ctx, c); err != nil {
			return []*types.Container{}, err
		}

		containers = append(containers, c)
	}
	return containers, nil
}

// WatchDeployStatus watch deployed status
func (k *Krypton) WatchDeployStatus(appname, entrypoint, nodename string) etcdclient.Watcher {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	key := filepath.Join(containerDeployPrefix, appname, entrypoint, nodename)
	return k.etcd.Watcher(key, &etcdclient.WatcherOptions{Recursive: true})
}

func (k *Krypton) bindNodeEngine(ctx context.Context, container *types.Container) (*types.Container, error) {
	node, err := k.GetNode(ctx, container.Podname, container.Nodename)
	if err != nil {
		return nil, err
	}

	container.Engine = node.Engine
	container.Node = node
	return container, nil
}
