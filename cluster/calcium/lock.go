package calcium

import (
	"context"
	"fmt"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout int) (lock.DistributedLock, error) {
	lock, err := c.store.CreateLock(name, timeout)
	if err != nil {
		return nil, err
	}
	if err = lock.Lock(ctx); err != nil {
		return nil, err
	}
	return lock, nil
}

func (c *Calcium) doUnlock(lock lock.DistributedLock, msg string) error {
	log.Debugf("[doUnlock] Unlock %s", msg)
	return lock.Unlock(context.Background())
}

func (c *Calcium) doUnlockAll(locks map[string]lock.DistributedLock) {
	for n, lock := range locks {
		// force unlock
		if err := c.doUnlock(lock, n); err != nil {
			log.Errorf("[doUnlockAll] Unlock failed %v", err)
			continue
		}
	}
}

func (c *Calcium) doLockAndGetContainer(ctx context.Context, ID string) (*types.Container, enginetypes.ContainerJSON, lock.DistributedLock, error) {
	lock, err := c.doLock(ctx, fmt.Sprintf(cluster.ContainerLock, ID), c.config.LockTimeout)
	if err != nil {
		return nil, enginetypes.ContainerJSON{}, nil, err
	}
	log.Debugf("[doLockAndGetContainer] Container %s locked", ID)
	// Get container
	container, err := c.store.GetContainer(ctx, ID)
	if err != nil {
		lock.Unlock(ctx)
		return nil, enginetypes.ContainerJSON{}, nil, err
	}
	// 确保是有这个容器的
	containerJSON, err := container.Inspect(ctx)
	if err != nil {
		lock.Unlock(ctx)
		return nil, enginetypes.ContainerJSON{}, nil, err
	}
	return container, containerJSON, lock, nil
}

func (c *Calcium) doLockAndGetContainers(ctx context.Context, IDs []string) (map[string]*types.Container, map[string]enginetypes.ContainerJSON, map[string]lock.DistributedLock, error) {
	containers := map[string]*types.Container{}
	containerJSONs := map[string]enginetypes.ContainerJSON{}
	locks := map[string]lock.DistributedLock{}
	for _, ID := range IDs {
		container, containerJSON, lock, err := c.doLockAndGetContainer(ctx, ID)
		if err != nil {
			c.doUnlockAll(locks)
			return nil, nil, nil, err
		}
		containers[ID] = container
		containerJSONs[ID] = containerJSON
		locks[ID] = lock
	}
	return containers, containerJSONs, locks, nil
}

func (c *Calcium) doLockAndGetNode(ctx context.Context, podname, nodename string) (*types.Node, lock.DistributedLock, error) {
	lock, err := c.doLock(ctx, fmt.Sprintf(cluster.NodeLock, podname, nodename), c.config.LockTimeout)
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("[doLockAndGetNode] Node %s locked", nodename)
	// Get node
	node, err := c.GetNode(ctx, podname, nodename)
	if err != nil {
		lock.Unlock(ctx)
		return nil, nil, err
	}
	return node, lock, nil
}

func (c *Calcium) doLockAndGetNodes(ctx context.Context, podname, nodename string, labels map[string]string) (map[string]*types.Node, map[string]lock.DistributedLock, error) {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}

	var ns []*types.Node
	var err error
	if nodename == "" {
		ns, err = c.ListPodNodes(ctx, podname, false)
		if err != nil {
			return nil, nil, err
		}
		nodeList := []*types.Node{}
		for _, node := range ns {
			if filterNode(node, labels) {
				nodeList = append(nodeList, node)
			}
		}
		ns = nodeList
	} else {
		n, err := c.GetNode(ctx, podname, nodename)
		if err != nil {
			return nil, nil, err
		}
		ns = append(ns, n)
	}
	if len(ns) == 0 {
		return nil, nil, types.ErrInsufficientNodes
	}

	for _, n := range ns {
		node, lock, err := c.doLockAndGetNode(ctx, podname, n.Name)
		if err != nil {
			c.doUnlockAll(locks)
			return nil, nil, err
		}
		nodes[node.Name] = node
		locks[node.Name] = lock
	}
	return nodes, locks, nil
}
