package calcium

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock lock.DistributedLock, rCtx context.Context, err error) {
	if lock, err = c.store.CreateLock(name, timeout); err != nil {
		return lock, rCtx, err
	}
	defer func() {
		if err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.TODO(), timeout)
			defer cancel()
			rollbackCtx = utils.InheritTracingInfo(rollbackCtx, ctx)
			if e := lock.Unlock(rollbackCtx); e != nil {
				log.WithFunc("calcium.doLock").Errorf(rollbackCtx, err, "failed to unlock %s", name)
			}
		}
	}()
	rCtx, err = lock.Lock(ctx)
	return lock, rCtx, err
}

func (c *Calcium) doUnlock(ctx context.Context, lock lock.DistributedLock, msg string) error {
	log.WithFunc("calcium.doUnlock").Debugf(ctx, "Unlock %s", msg)
	return lock.Unlock(ctx)
}

func (c *Calcium) doUnlockAll(ctx context.Context, locks map[string]lock.DistributedLock, order ...string) {
	logger := log.WithFunc("calcium.doUnlockAll")
	// unlock in the reverse order
	if len(order) != len(locks) {
		logger.Warn(ctx, "order length not match lock map")
		order = []string{}
		for key := range locks {
			order = append(order, key)
		}
	}
	for _, key := range order {
		if err := c.doUnlock(ctx, locks[key], key); err != nil {
			logger.Errorf(ctx, err, "Unlock %s failed", key)
			continue
		}
	}
}

func (c *Calcium) withWorkloadLocked(ctx context.Context, id string, f func(context.Context, *types.Workload) error) error {
	return c.withWorkloadsLocked(ctx, []string{id}, func(ctx context.Context, workloads map[string]*types.Workload) error {
		if c, ok := workloads[id]; ok {
			return f(ctx, c)
		}
		return types.ErrWorkloadNotExists
	})
}

func (c *Calcium) withWorkloadsLocked(ctx context.Context, ids []string, f func(context.Context, map[string]*types.Workload) error) error {
	workloads := map[string]*types.Workload{}
	locks := map[string]lock.DistributedLock{}
	logger := log.WithFunc("calcium.withWorkloadsLocked")

	// sort + unique
	sort.Strings(ids)
	ids = ids[:utils.Unique(ids, func(i int) string { return ids[i] })]

	defer logger.Debugf(ctx, "Workloads %+v unlocked", ids)
	defer func() {
		utils.Reverse(ids)
		c.doUnlockAll(utils.InheritTracingInfo(ctx, context.TODO()), locks, ids...)
	}()
	cs, err := c.store.GetWorkloads(ctx, ids)
	if err != nil {
		return err
	}
	var lock lock.DistributedLock
	for _, workload := range cs {
		lock, ctx, err = c.doLock(ctx, fmt.Sprintf(cluster.WorkloadLock, workload.ID), c.config.LockTimeout)
		if err != nil {
			return err
		}
		logger.Debugf(ctx, "Workload %s locked", workload.ID)
		locks[workload.ID] = lock
		workloads[workload.ID] = workload
	}
	return f(ctx, workloads)
}

func (c *Calcium) withNodePodLocked(ctx context.Context, nodename string, f func(context.Context, *types.Node) error) error {
	nodeFilter := &types.NodeFilter{
		Includes: []string{nodename},
		All:      true,
	}
	return c.withNodesPodLocked(ctx, nodeFilter, func(ctx context.Context, nodes map[string]*types.Node) error {
		if n, ok := nodes[nodename]; ok {
			return f(ctx, n)
		}
		return types.ErrNodeNotExists
	})
}

func (c *Calcium) withNodeOperationLocked(ctx context.Context, nodename string, f func(context.Context, *types.Node) error) error {
	nodeFilter := &types.NodeFilter{
		Includes: []string{nodename},
		All:      true,
	}
	return c.withNodesOperationLocked(ctx, nodeFilter, func(ctx context.Context, nodes map[string]*types.Node) error {
		if n, ok := nodes[nodename]; ok {
			return f(ctx, n)
		}
		return types.ErrNodeNotExists
	})
}

func (c *Calcium) withNodesOperationLocked(ctx context.Context, nodeFilter *types.NodeFilter, f func(context.Context, map[string]*types.Node) error) error { //nolint
	genKey := func(node *types.Node) string {
		return fmt.Sprintf(cluster.NodeOperationLock, node.Podname, node.Name)
	}
	return c.withNodesLocked(ctx, nodeFilter, genKey, f)
}

func (c *Calcium) withNodesPodLocked(ctx context.Context, nodeFilter *types.NodeFilter, f func(context.Context, map[string]*types.Node) error) error {
	genKey := func(node *types.Node) string {
		return fmt.Sprintf(cluster.PodLock, node.Podname)
	}
	return c.withNodesLocked(ctx, nodeFilter, genKey, f)
}

func (c *Calcium) withNodesLocked(ctx context.Context, nodeFilter *types.NodeFilter, genKey func(*types.Node) string, f func(context.Context, map[string]*types.Node) error) error {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	lockKeys := []string{}
	logger := log.WithFunc("calcium.withNodesLocked")

	defer func() {
		utils.Reverse(lockKeys)
		c.doUnlockAll(utils.InheritTracingInfo(ctx, context.TODO()), locks, lockKeys...)
		logger.Debugf(ctx, "keys %+v unlocked", lockKeys)
	}()

	ns, err := c.filterNodes(ctx, nodeFilter)
	if err != nil {
		return err
	}

	var lock lock.DistributedLock
	for _, n := range ns {
		key := genKey(n)
		if _, ok := locks[key]; !ok {
			lock, ctx, err = c.doLock(ctx, key, c.config.LockTimeout)
			if err != nil {
				return err
			}
			logger.Debugf(ctx, "key %s locked", key)
			locks[key] = lock
			lockKeys = append(lockKeys, key)
		}
		nodes[n.Name] = n
	}
	return f(ctx, nodes)
}
