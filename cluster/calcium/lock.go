package calcium

import (
	"fmt"
	"time"
	"context"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock.DistributedLock, error) {
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

func (c *Calcium) withContainerLocked(ctx context.Context, ID string, f func(container *types.Container) error) error {
	lock, err := c.doLock(ctx, fmt.Sprintf(cluster.ContainerLock, ID), c.config.LockTimeout)
	if err != nil {
		return err
	}
	defer c.doUnlock(lock, ID)
	log.Debugf("[withContainerLocked] Container %s locked", ID)
	// Get container meta
	container, err := c.store.GetContainer(ctx, ID)
	if err != nil {
		return err
	}
	return f(container)
}

func (c *Calcium) withNodeLocked(ctx context.Context, podname, nodename string, f func(node *types.Node) error) error {
	lock, err := c.doLock(ctx, fmt.Sprintf(cluster.NodeLock, podname, nodename), c.config.LockTimeout)
	if err != nil {
		return err
	}
	defer c.doUnlock(lock, nodename)
	log.Debugf("[withNodeLocked] Node %s locked", nodename)
	// Get node
	node, err := c.GetNode(ctx, podname, nodename)
	if err != nil {
		return err
	}
	return f(node)
}

func (c *Calcium) withContainersLocked(ctx context.Context, IDs []string, f func(containers map[string]*types.Container) error) error {
	containers := map[string]*types.Container{}
	locks := map[string]lock.DistributedLock{}
	defer func() { c.doUnlockAll(locks) }()
	for _, ID := range IDs {
		if _, ok := containers[ID]; ok {
			continue
		}
		lock, err := c.doLock(ctx, fmt.Sprintf(cluster.ContainerLock, ID), c.config.LockTimeout)
		if err != nil {
			return err
		}
		log.Debugf("[withContainersLocked] Container %s locked", ID)
		locks[ID] = lock
		container, err := c.store.GetContainer(ctx, ID)
		if err != nil {
			return err
		}
		containers[ID] = container
	}
	return f(containers)
}

func (c *Calcium) withNodesLocked(ctx context.Context, podname, nodename string, labels map[string]string, f func(nodes map[string]*types.Node) error) error {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	defer func() { c.doUnlockAll(locks) }()

	var ns []*types.Node
	var err error
	if nodename == "" {
		// TODO should consider all nodes
		ns, err = c.ListPodNodes(ctx, podname, false)
		if err != nil {
			return err
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
			return err
		}
		ns = append(ns, n)
	}

	for _, n := range ns {
		lock, err := c.doLock(ctx, fmt.Sprintf(cluster.NodeLock, podname, n.Name), c.config.LockTimeout)
		if err != nil {
			return err
		}
		log.Debugf("[withNodesLocked] Node %s locked", n.Name)
		locks[n.Name] = lock
		// refresh node
		node, err := c.GetNode(ctx, podname, n.Name)
		if err != nil {
			return err
		}
		nodes[n.Name] = node
	}
	return f(nodes)
}
