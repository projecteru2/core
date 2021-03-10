package calcium

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock.DistributedLock, context.Context, error) {
	lock, err := c.store.CreateLock(name, timeout)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	ctx, err = lock.Lock(ctx)
	return lock, ctx, errors.WithStack(err)
}

func (c *Calcium) doUnlock(ctx context.Context, lock lock.DistributedLock, msg string) error {
	log.Debugf("[doUnlock] Unlock %s", msg)
	return errors.WithStack(lock.Unlock(ctx))
}

func (c *Calcium) doUnlockAll(ctx context.Context, locks map[string]lock.DistributedLock) {
	for n, lock := range locks {
		// force unlock
		if err := c.doUnlock(ctx, lock, n); err != nil {
			log.Errorf("[doUnlockAll] Unlock failed %v", err)
			continue
		}
	}
}

func (c *Calcium) withWorkloadLocked(ctx context.Context, id string, f func(context.Context, *types.Workload) error) error {
	return c.withWorkloadsLocked(ctx, []string{id}, func(ctx context.Context, workloads map[string]*types.Workload) error {
		if c, ok := workloads[id]; ok {
			return f(ctx, c)
		}
		return errors.WithStack(types.ErrWorkloadNotExists)
	})
}

func (c *Calcium) withNodeLocked(ctx context.Context, nodename string, f func(context.Context, *types.Node) error) error {
	return c.withNodesLocked(ctx, "", []string{nodename}, nil, true, func(ctx context.Context, nodes map[string]*types.Node) error {
		if n, ok := nodes[nodename]; ok {
			return f(ctx, n)
		}
		return errors.WithStack(types.ErrNodeNotExists)
	})
}

func (c *Calcium) withWorkloadsLocked(ctx context.Context, ids []string, f func(context.Context, map[string]*types.Workload) error) error {
	workloads := map[string]*types.Workload{}
	locks := map[string]lock.DistributedLock{}
	defer log.Debugf("[withWorkloadsLocked] Workloads %+v unlocked", ids)
	defer func() { c.doUnlockAll(context.Background(), locks) }()
	cs, err := c.GetWorkloads(ctx, ids)
	if err != nil {
		return err
	}
	var lock lock.DistributedLock
	for _, workload := range cs {
		lock, ctx, err = c.doLock(ctx, fmt.Sprintf(cluster.WorkloadLock, workload.ID), c.config.LockTimeout)
		if err != nil {
			return errors.WithStack(err)
		}
		log.Debugf("[withWorkloadsLocked] Workload %s locked", workload.ID)
		locks[workload.ID] = lock
		workloads[workload.ID] = workload
	}
	return f(ctx, workloads)
}

func (c *Calcium) withNodesLocked(ctx context.Context, podname string, nodenames []string, labels map[string]string, all bool, f func(context.Context, map[string]*types.Node) error) error {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	defer log.Debugf("[withNodesLocked] Nodes %+v unlocked", nodenames)
	defer c.doUnlockAll(context.Background(), locks)
	ns, err := c.getNodes(ctx, podname, nodenames, labels, all)
	if err != nil {
		return err
	}

	var lock lock.DistributedLock
	for _, n := range ns {
		lock, ctx, err = c.doLock(ctx, fmt.Sprintf(cluster.NodeLock, podname, n.Name), c.config.LockTimeout)
		if err != nil {
			return err
		}
		log.Debugf("[withNodesLocked] Node %s locked", n.Name)
		locks[n.Name] = lock
		// refresh node
		node, err := c.GetNode(ctx, n.Name)
		if err != nil {
			return err
		}
		nodes[n.Name] = node
	}
	return f(ctx, nodes)
}
