package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) RemoveWorkload(ctx context.Context, IDs []string, force bool) (chan *types.RemoveWorkloadMessage, error) {
	logger := log.WithFunc("calcium.RemoveWorkload").WithField("IDs", IDs).WithField("force", force)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, IDs)
	if err != nil {
		logger.Error(ctx, err, "failed to group workloads by node")
		return nil, err
	}

	ch := make(chan *types.RemoveWorkloadMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for nodename, workloadIDs := range nodeWorkloadGroup {
			wg.Add(1)
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				if nodeErr := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
					for _, workloadID := range workloadIDs {
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
						ch <- ret
					}
					_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })
					return nil
				}); nodeErr != nil {
					logger.WithField("node", nodename).Error(ctx, nodeErr, "failed to lock node")
					ch <- &types.RemoveWorkloadMessage{Success: false}
				}
			})
		}
	})
	return ch, nil
}

func (c *Calcium) RemoveWorkloadSync(ctx context.Context, IDs []string) error {
	return c.doRemoveWorkloadSync(ctx, IDs)
}

func (c *Calcium) doRemoveOneWorkload(ctx context.Context, node *types.Node, workload *types.Workload, force bool) error {
	logger := log.WithFunc("calcium.doRemoveOneWorkload").WithField("id", workload.ID)

	nodeCommit, err := c.wal.Log(eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	defer func() {
		if commitErr := nodeCommit(); commitErr != nil {
			logger.Errorf(ctx, commitErr, "commit wal failed: %s", eventWorkloadResourceAllocated)
		}
	}()

	workloadCommit, err := c.wal.Log(eventWorkloadCreated, &types.Workload{ID: workload.ID, Nodename: workload.Nodename})
	if err != nil {
		return err
	}
	defer func() {
		if commitErr := workloadCommit(); commitErr != nil {
			logger.Errorf(ctx, commitErr, "commit wal failed: %s", eventWorkloadCreated)
		}
	}()

	return c.withResourceReleased(ctx, node, workload, func(ctx context.Context) error {
		return c.doRemoveWorkload(ctx, workload, force)
	})
}

func (c *Calcium) doRemoveWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	return utils.Txn(
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
