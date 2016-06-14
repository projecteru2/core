package etcdstore

import (
	"encoding/json"
	"fmt"

	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

// get a container
// container if must be in full length, or we can't find it in etcd
// storage path in etcd is `/eru-core/container/:containerid`
func (k *Krypton) GetContainer(id string) (*types.Container, error) {
	if len(id) < 64 {
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

// add a container
// mainly record its relationship on pod and node
// actually if we already know its node, we will know its pod
// but we still store it
// storage path in etcd is `/eru-core/container/:containerid`
func (k *Krypton) AddContainer(id, podname, nodename string) (*types.Container, error) {
	// first we check if node really exists
	node, err := k.GetNode(podname, nodename)
	if err != nil {
		return nil, err
	}

	// then we check if container really exists
	c, err := node.Engine.ContainerInspect(context.Background(), id)
	if err != nil {
		return nil, err
	}

	// now everything is ok
	// we use full length id instead
	key := fmt.Sprintf(containerInfoKey, id)
	container := &types.Container{
		ID:       c.ID,
		Podname:  podname,
		Nodename: nodename,
		Engine:   node.Engine,
	}

	bytes, err := json.Marshal(container)
	if err != nil {
		return nil, err
	}

	_, err := k.etcd.Set(context.Background(), key, string(bytes), nil)
	if err != nil {
		return nil, err
	}

	return container, nil
}

// remove a container
// container id must be in full length
func (k *Krypton) RemoveContainer(id string) error {
	if len(id) < 64 {
		return fmt.Errorf("Container ID must be length of 64")
	}

	key := fmt.Sprintf(containerInfoKey, id)
	_, err := k.etcd.Delete(context.Background(), key, nil)
	return err
}
