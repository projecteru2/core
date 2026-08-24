package calcium

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type (
	nodeHandler      func(context.Context, *types.Node) error
	nodesHandler     func(context.Context, map[string]*types.Node) error
	workloadHandler  func(context.Context, *types.Workload) error
	workloadsHandler func(context.Context, map[string]*types.Workload) error
	nodeLockKey      func(*types.Node) string
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock lock.DistributedLock, rCtx context.Context, err error) {
	if lock, err = c.store.CreateLock(name, timeout); err != nil {
		return lock, ctx, err
	}
	defer func() {
		if err != nil {
			rollbackCtx, cancel := context.WithTimeout(utils.NewInheritCtx(ctx), timeout)
			defer cancel()
			if e := lock.Unlock(rollbackCtx); e != nil {
				log.WithFunc("calcium.doLock").Errorf(rollbackCtx, e, "failed to unlock %s", name)
			}
		}
	}()
	rCtx, err = lock.Lock(ctx)
	return lock, rCtx, err
}

func (c *Calcium) doUnlock(ctx context.Context, lock lock.DistributedLock, msg string) error {
	log.WithFunc("calcium.doUnlock").Debugf(ctx, "unlock %s", msg)
	return lock.Unlock(ctx)
}

func (c *Calcium) doUnlockAll(ctx context.Context, locks map[string]lock.DistributedLock, order ...string) {
	logger := log.WithFunc("calcium.doUnlockAll")
	if len(order) != len(locks) {
		logger.Warn(ctx, "order length does not match lock map")
		order = []string{}
		for key := range locks {
			order = append(order, key)
		}
	}
	for _, key := range order {
		if err := c.doUnlock(ctx, locks[key], key); err != nil {
			logger.Errorf(ctx, err, "failed to unlock %s", key)
			continue
		}
	}
}

func (c *Calcium) withWorkloadLocked(ctx context.Context, ID string, ignoreLock bool, f workloadHandler) error {
	return c.withWorkloadsLocked(ctx, ignoreLock, []string{ID}, func(ctx context.Context, workloads map[string]*types.Workload) error {
		if c, ok := workloads[ID]; ok {
			return f(ctx, c)
		}
		return types.ErrWorkloadNotExists
	})
}

func (c *Calcium) withWorkloadsLocked(ctx context.Context, ignoreLock bool, IDs []string, f workloadsHandler) error {
	workloads := map[string]*types.Workload{}
	locks := map[string]lock.DistributedLock{}
	lockKeys := []string{}
	logger := log.WithFunc("calcium.withWorkloadsLocked")

	slices.Sort(IDs)
	IDs = slices.Compact(IDs)

	defer func() {
		slices.Reverse(lockKeys)
		c.doUnlockAll(utils.NewInheritCtx(ctx), locks, lockKeys...)
		logger.Debugf(ctx, "workloads %+v unlocked", lockKeys)
	}()
	cs, err := c.store.GetWorkloads(ctx, IDs)
	if err != nil {
		return err
	}
	var lock lock.DistributedLock
	for _, workload := range cs {
		if !ignoreLock {
			lock, ctx, err = c.doLock(ctx, fmt.Sprintf(cluster.WorkloadLock, workload.ID), c.config.LockTimeout)
			if err != nil {
				return err
			}
			logger.Debugf(ctx, "workload %s locked", workload.ID)
			locks[workload.ID] = lock
			lockKeys = append(lockKeys, workload.ID)
		}
		workloads[workload.ID] = workload
	}
	return f(ctx, workloads)
}

func (c *Calcium) withNodePodLocked(ctx context.Context, nodename string, f nodeHandler) error {
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

func (c *Calcium) withNodeOperationLocked(ctx context.Context, nodename string, f nodeHandler) error {
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

func (c *Calcium) withNodesOperationLocked(ctx context.Context, nodeFilter *types.NodeFilter, f nodesHandler) error { //nolint:unused
	genKey := func(node *types.Node) string {
		return fmt.Sprintf(cluster.NodeOperationLock, node.Podname, node.Name)
	}
	return c.withNodesLocked(ctx, nodeFilter, genKey, f)
}

func (c *Calcium) withNodesPodLocked(ctx context.Context, nodeFilter *types.NodeFilter, f nodesHandler) error {
	genKey := func(node *types.Node) string {
		return fmt.Sprintf(cluster.PodLock, node.Podname)
	}
	return c.withNodesLocked(ctx, nodeFilter, genKey, f)
}

func (c *Calcium) withNodesLocked(ctx context.Context, nodeFilter *types.NodeFilter, genKey nodeLockKey, f nodesHandler) error {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	lockKeys := []string{}
	logger := log.WithFunc("calcium.withNodesLocked")

	defer func() {
		slices.Reverse(lockKeys)
		c.doUnlockAll(utils.NewInheritCtx(ctx), locks, lockKeys...)
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
