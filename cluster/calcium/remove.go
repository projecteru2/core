package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// removeWorkers bounds the engine removes in flight on one node.
const removeWorkers = 16

func (c *Calcium) RemoveWorkload(ctx context.Context, IDs []string, force bool) (chan *types.RemoveWorkloadMessage, error) {
	logger := log.WithFunc("calcium.RemoveWorkload").WithField("IDs", IDs).WithField("force", force)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, IDs)
	if err != nil {
		logger.Error(ctx, err, "failed to group workloads by node")
		return nil, err
	}

	ch := make(chan *types.RemoveWorkloadMessage)
	caller := ctx
	utils.SentryGo(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for nodename, workloadIDs := range nodeWorkloadGroup {
			wg.Add(1)
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				node, err := c.store.GetNode(ctx, nodename)
				if err != nil {
					logger.WithField("node", nodename).Error(ctx, err, "failed to get node")
					_ = send(caller, ch, &types.RemoveWorkloadMessage{Success: false})
					return
				}
				var removes errgroup.Group
				removes.SetLimit(removeWorkers)
				for _, workloadID := range workloadIDs {
					removes.Go(func() error {
						defer log.SentryDefer()
						ret := &types.RemoveWorkloadMessage{WorkloadID: workloadID, Success: true, Hook: []*bytes.Buffer{}}
						if workloadErr := c.withWorkloadLocked(ctx, workloadID, false, func(ctx context.Context, workload *types.Workload) error {
							if err := c.doRemoveOneWorkload(ctx, node, workload, force); err != nil {
								return err
							}
							logger.Infof(ctx, "workload %s removed", workload.ID)
							return nil
						}); workloadErr != nil {
							logger.WithField("id", workloadID).Error(ctx, workloadErr, "failed to remove workload")
							ret.Hook = append(ret.Hook, bytes.NewBufferString(workloadErr.Error()))
							ret.Success = false
						}
						_ = send(caller, ch, ret)
						return nil
					})
				}
				_ = removes.Wait()
				c.invokePoolAsync(func() { c.RemapResourceAndLog(ctx, logger, node) })
			})
		}
	})
	return ch, nil
}

func (c *Calcium) doRemoveOneWorkload(ctx context.Context, node *types.Node, workload *types.Workload, force bool) error {
	logger := log.WithFunc("calcium.doRemoveOneWorkload").WithField("id", workload.ID)

	nodeCommit, err := c.journal(ctx, logger, eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	defer nodeCommit()

	workloadCommit, err := c.journal(ctx, logger, eventWorkloadCreated, &types.Workload{ID: workload.ID, Name: workload.Name, Nodename: workload.Nodename})
	if err != nil {
		return err
	}
	defer workloadCommit()

	return c.withResourceReleased(ctx, node, workload, func(ctx context.Context) error {
		return c.doRemoveWorkload(ctx, workload, force)
	})
}

func (c *Calcium) doRemoveWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	_, err := utils.Txn(
		ctx,
		func(ctx context.Context) error {
			return c.store.RemoveWorkload(ctx, workload)
		},
		func(ctx context.Context) error {
			return workload.Remove(ctx, force)
		},
		func(ctx context.Context, failedByCond bool) error {
			if failedByCond {
				return nil
			}
			return c.store.AddWorkload(ctx, workload, nil)
		},
		c.config.GlobalTimeout,
	)
	return err
}

func (c *Calcium) doRemoveWorkloadSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveWorkload(ctx, IDs, true)
	if err != nil {
		return err
	}

	logger := log.WithFunc("calcium.doRemoveWorkloadSync")
	errs := []error{}
	for m := range ch {
		if !m.Success {
			errs = append(errs, errors.Newf("failed to remove workload %s: %s", m.WorkloadID, utils.MergeHookOutputs(m.Hook)))
			continue
		}
		logger.Debugf(ctx, "removed %s", m.WorkloadID)
	}
	return errors.Join(errs...)
}

func (c *Calcium) groupWorkloadsByNode(ctx context.Context, IDs []string) (map[string][]string, error) {
	workloads, err := c.store.GetWorkloads(ctx, IDs)
	if err != nil {
		return nil, err
	}
	nodeWorkloadGroup := map[string][]string{}
	for _, workload := range workloads {
		nodeWorkloadGroup[workload.Nodename] = append(nodeWorkloadGroup[workload.Nodename], workload.ID)
	}
	return nodeWorkloadGroup, nil
}
