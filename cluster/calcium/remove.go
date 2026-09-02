package calcium

import (
	"bytes"
	"context"
	"maps"
	"slices"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	resourcetypes "github.com/projecteru2/core/resource/types"
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
	utils.SentryGo(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for nodename, workloadIDs := range nodeWorkloadGroup {
			wg.Add(1)
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				if nodeErr := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
					return c.doRemoveNodeWorkloads(ctx, node, workloadIDs, force, ch)
				}); nodeErr != nil {
					logger.WithField("node", nodename).Error(ctx, nodeErr, "failed to remove the workloads of the node")
					for _, workloadID := range workloadIDs {
						select {
						case ch <- &types.RemoveWorkloadMessage{WorkloadID: workloadID, Success: false, Hook: []*bytes.Buffer{bytes.NewBufferString(nodeErr.Error())}}:
						case <-ctx.Done():
							return
						}
					}
				}
			})
		}
	})
	return ch, nil
}

func (c *Calcium) doRemoveNodeWorkloads(ctx context.Context, node *types.Node, IDs []string, force bool, ch chan<- *types.RemoveWorkloadMessage) error {
	logger := log.WithFunc("calcium.doRemoveNodeWorkloads").WithField("node", node.Name)
	workloads, err := c.store.GetWorkloads(ctx, IDs)
	if err != nil {
		return err
	}
	nodeCommit, err := c.journal(ctx, logger, eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	defer nodeCommit()

	staying := make(map[string]resourcetypes.Resources, len(workloads))
	resources := make([]resourcetypes.Resources, 0, len(workloads))
	for _, workload := range workloads {
		staying[workload.ID] = workload.Resources
		resources = append(resources, workload.Resources)
	}
	if _, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, resources, true, plugins.Decr); err != nil {
		return err
	}

	defer func() {
		if len(staying) == 0 {
			return
		}
		restoreCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), c.config.GlobalTimeout)
		defer cancel()
		if _, _, err := c.rmgr.SetNodeResourceUsage(restoreCtx, node.Name, nil, nil, slices.Collect(maps.Values(staying)), true, plugins.Incr); err != nil {
			logger.Error(ctx, err, "failed to give the workloads that stay their resources back")
		}
	}()
	for _, workload := range workloads {
		ret := &types.RemoveWorkloadMessage{WorkloadID: workload.ID, Success: true, Hook: []*bytes.Buffer{}}
		if workloadErr := c.withWorkloadLocked(ctx, workload.ID, false, func(ctx context.Context, workload *types.Workload) error {
			return c.doRemoveOneWorkload(ctx, workload, force)
		}); workloadErr != nil {
			logger.WithField("id", workload.ID).Error(ctx, workloadErr, "failed to remove workload")
			ret.Hook = append(ret.Hook, bytes.NewBufferString(workloadErr.Error()))
			ret.Success = false
		} else {
			delete(staying, workload.ID)
			logger.Infof(ctx, "workload %s removed", workload.ID)
		}
		select {
		case ch <- ret:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	c.invokePoolAsync(func() { c.RemapResourceAndLog(ctx, logger, node) })
	return nil
}

func (c *Calcium) doRemoveOneWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	logger := log.WithFunc("calcium.doRemoveOneWorkload").WithField("id", workload.ID)
	workloadCommit, err := c.journal(ctx, logger, eventWorkloadCreated, &types.Workload{ID: workload.ID, Name: workload.Name, Nodename: workload.Nodename})
	if err != nil {
		return err
	}
	defer workloadCommit()
	return c.doRemoveWorkload(ctx, workload, force)
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
