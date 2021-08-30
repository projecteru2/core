package calcium

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) doLock(ctx context.Context, name string, timeout time.Duration) (lock lock.DistributedLock, rCtx context.Context, err error) {
	if lock, err = c.store.CreateLock(name, timeout); err != nil {
		return lock, rCtx, errors.WithStack(err)
	}
	defer func() {
		if err != nil {
			rollbackCtx, cancel := context.WithTimeout(context.TODO(), timeout)
			defer cancel()
			rollbackCtx = utils.InheritTracingInfo(rollbackCtx, ctx)
			if e := lock.Unlock(rollbackCtx); e != nil {
				log.Errorf(rollbackCtx, "failed to unlock %s: %+v", name, err)
			}
		}
	}()
	rCtx, err = lock.Lock(ctx)
	return lock, rCtx, errors.WithStack(err)

}

func (c *Calcium) doUnlock(ctx context.Context, lock lock.DistributedLock, msg string) error {
	log.Debugf(ctx, "[doUnlock] Unlock %s", msg)
	return errors.WithStack(lock.Unlock(ctx))
}

func (c *Calcium) doUnlockAll(ctx context.Context, locks map[string]lock.DistributedLock, order ...string) {
	// unlock in the reverse order
	if len(order) != len(locks) {
		log.Warn(ctx, "[doUnlockAll] order length not match lock map")
		order = []string{}
		for key := range locks {
			order = append(order, key)
		}
	}
	for _, key := range order {
		if err := c.doUnlock(ctx, locks[key], key); err != nil {
			log.Errorf(ctx, "[doUnlockAll] Unlock %s failed %v", key, err)
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
	nf := types.NodeFilter{
		Includes: []string{nodename},
		All:      true,
	}
	return c.withNodesLocked(ctx, nf, func(ctx context.Context, nodes map[string]*types.Node) error {
		if n, ok := nodes[nodename]; ok {
			return f(ctx, n)
		}
		return errors.WithStack(types.ErrNodeNotExists)
	})
}

func (c *Calcium) withWorkloadsLocked(ctx context.Context, ids []string, f func(context.Context, map[string]*types.Workload) error) error {
	workloads := map[string]*types.Workload{}
	locks := map[string]lock.DistributedLock{}

	// sort + unique
	sort.Strings(ids)
	ids = ids[:utils.Unique(ids, func(i int) string { return ids[i] })]

	defer log.Debugf(ctx, "[withWorkloadsLocked] Workloads %+v unlocked", ids)
	defer func() {
		utils.Reverse(ids)
		c.doUnlockAll(utils.InheritTracingInfo(ctx, context.TODO()), locks, ids...)
	}()
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
		log.Debugf(ctx, "[withWorkloadsLocked] Workload %s locked", workload.ID)
		locks[workload.ID] = lock
		workloads[workload.ID] = workload
	}
	return f(ctx, workloads)
}

// withNodesLocked will using NodeFilter `nf` to filter nodes
// and lock the corresponding nodes for the callback function `f` to use
func (c *Calcium) withNodesLocked(ctx context.Context, nf types.NodeFilter, f func(context.Context, map[string]*types.Node) error) error {
	nodenames := []string{}
	nodes := map[string]*types.Node{}
	locks := map[string]lock.DistributedLock{}
	defer log.Debugf(ctx, "[withNodesLocked] Nodes %+v unlocked", nf)
	defer func() {
		utils.Reverse(nodenames)
		c.doUnlockAll(utils.InheritTracingInfo(ctx, context.TODO()), locks, nodenames...)
	}()

	ns, err := c.filterNodes(ctx, nf)
	if err != nil {
		return err
	}

	var lock lock.DistributedLock
	for _, n := range ns {
		lock, ctx, err = c.doLock(ctx, fmt.Sprintf(cluster.NodeLock, n.Podname, n.Name), c.config.LockTimeout)
		if err != nil {
			return err
		}
		log.Debugf(ctx, "[withNodesLocked] Node %s locked", n.Name)
		locks[n.Name] = lock
		// refresh node
		node, err := c.GetNode(ctx, n.Name)
		if err != nil {
			return err
		}
		nodes[n.Name] = node
		nodenames = append(nodenames, n.Name)
	}
	return f(ctx, nodes)
}
