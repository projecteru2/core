package calcium

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type (
	nodeHandler     func(context.Context, *types.Node) error
	nodesHandler    func(context.Context, map[string]*types.Node) error
	workloadHandler func(context.Context, *types.Workload) error
	nodeLockKeys    func([]*types.Node) []string
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock lock.DistributedLock, rCtx context.Context, err error) {
	if lock, err = c.store.CreateLock(name, timeout); err != nil {
		return lock, ctx, err
	}
	defer func() {
		if err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), timeout)
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

func (c *Calcium) doUnlockAll(ctx context.Context, locks map[string]lock.DistributedLock, order []string) {
	logger := log.WithFunc("calcium.doUnlockAll")
	for _, key := range order {
		if err := c.doUnlock(ctx, locks[key], key); err != nil {
			logger.Errorf(ctx, err, "failed to unlock %s", key)
		}
	}
}

func (c *Calcium) withWorkloadLocked(ctx context.Context, ID string, ignoreLock bool, f workloadHandler) error {
	workload, err := c.store.GetWorkload(ctx, ID)
	if err != nil {
		return err
	}
	if ignoreLock {
		return f(ctx, workload)
	}

	logger := log.WithFunc("calcium.withWorkloadLocked")
	lock, ctx, err := c.doLock(ctx, fmt.Sprintf(cluster.WorkloadLock, workload.ID), c.config.LockTimeout)
	if err != nil {
		return err
	}
	logger.Debugf(ctx, "workload %s locked", workload.ID)
	defer func() {
		if err := c.doUnlock(context.WithoutCancel(ctx), lock, workload.ID); err != nil {
			logger.Errorf(ctx, err, "failed to unlock workload %s", workload.ID)
		}
	}()
	return f(ctx, workload)
}

func (c *Calcium) withNodeOperationLocked(ctx context.Context, nodename string, f nodeHandler) error {
	return withNodeLocked(ctx, nodename, c.withNodesOperationLocked, f)
}

func (c *Calcium) withNodesOperationLocked(ctx context.Context, nodeFilter *types.NodeFilter, f nodesHandler) error {
	return c.withNodesLocked(ctx, nodeFilter, nodeOperationKeys, f)
}

// withNodesPlanLocked serializes a deploy plan: one candidate takes its node lock, several take the pod lock and every node lock.
func (c *Calcium) withNodesPlanLocked(ctx context.Context, nodeFilter *types.NodeFilter, f nodesHandler) error {
	return c.withNodesLocked(ctx, nodeFilter, func(nodes []*types.Node) []string {
		if len(nodes) == 1 {
			return nodeOperationKeys(nodes)
		}
		return append(podKeys(nodes), nodeOperationKeys(nodes)...)
	}, f)
}

func (c *Calcium) withPodLocked(ctx context.Context, podname string, f nodesHandler) error {
	return c.withNodesLocked(ctx, &types.NodeFilter{Podname: podname, All: true}, podKeys, f)
}

func (c *Calcium) withNodesLocked(ctx context.Context, nodeFilter *types.NodeFilter, keysOf nodeLockKeys, f nodesHandler) error {
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	lockKeys := []string{}
	logger := log.WithFunc("calcium.withNodesLocked")

	defer func() {
		slices.Reverse(lockKeys)
		c.doUnlockAll(context.WithoutCancel(ctx), locks, lockKeys)
		logger.Debugf(ctx, "keys %+v unlocked", lockKeys)
	}()

	ns, err := c.filterNodes(ctx, nodeFilter)
	if err != nil {
		return err
	}
	for _, n := range ns {
		nodes[n.Name] = n
	}

	keys := keysOf(ns)
	slices.SortFunc(keys, byLockOrder)
	var lock lock.DistributedLock
	for _, key := range slices.Compact(keys) {
		lock, ctx, err = c.doLock(ctx, key, c.config.LockTimeout)
		if err != nil {
			return err
		}
		logger.Debugf(ctx, "key %s locked", key)
		locks[key] = lock
		lockKeys = append(lockKeys, key)
	}
	return f(ctx, nodes)
}

// withNodeKeyLocked takes one node's operation lock around f; the caller already holds the node.
func (c *Calcium) withNodeKeyLocked(ctx context.Context, node *types.Node, f func(context.Context) error) error {
	lock, ctx, err := c.doLock(ctx, nodeOperationKey(node), c.config.LockTimeout)
	if err != nil {
		return err
	}
	defer func() {
		if err := c.doUnlock(context.WithoutCancel(ctx), lock, nodeOperationKey(node)); err != nil {
			log.WithFunc("calcium.withNodeKeyLocked").Errorf(ctx, err, "failed to unlock %s", nodeOperationKey(node))
		}
	}()
	return f(ctx)
}

// withNodes resolves the filter without taking any lock, for paths that only read.
func (c *Calcium) withNodes(ctx context.Context, nodeFilter *types.NodeFilter, f nodesHandler) error {
	ns, err := c.filterNodes(ctx, nodeFilter)
	if err != nil {
		return err
	}
	nodes := make(map[string]*types.Node, len(ns))
	for _, n := range ns {
		nodes[n.Name] = n
	}
	return f(ctx, nodes)
}

func (c *Calcium) withNode(ctx context.Context, nodename string, f nodeHandler) error {
	return withNodeLocked(ctx, nodename, c.withNodes, f)
}

func nodeOperationKey(node *types.Node) string {
	return fmt.Sprintf(cluster.NodeOperationLock, node.Podname, node.Name)
}

func nodeOperationKeys(nodes []*types.Node) []string {
	return utils.Map(nodes, nodeOperationKey)
}

func podKeys(nodes []*types.Node) []string {
	return utils.Map(nodes, func(node *types.Node) string { return fmt.Sprintf(cluster.PodLock, node.Podname) })
}

func withNodeLocked(ctx context.Context, nodename string, withNodes func(context.Context, *types.NodeFilter, nodesHandler) error, f nodeHandler) error {
	nodeFilter := &types.NodeFilter{
		Includes: []string{nodename},
		All:      true,
	}
	return withNodes(ctx, nodeFilter, func(ctx context.Context, nodes map[string]*types.Node) error {
		if n, ok := nodes[nodename]; ok {
			return f(ctx, n)
		}
		return types.ErrNodeNotExists
	})
}

// byLockOrder puts pod locks before node locks, so a plan blocks a node only for as long as it plans on it.
func byLockOrder(a, b string) int {
	rank := func(key string) int {
		if strings.HasPrefix(key, "plock_") {
			return 0
		}
		return 1
	}
	return cmp.Or(cmp.Compare(rank(a), rank(b)), strings.Compare(a, b))
}
