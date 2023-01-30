package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// RemoveWorkload remove workloads
// returns a channel that contains removing responses
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
			_ = c.pool.Invoke(func(nodename string, workloadIDs []string) func() {
				return func() {
					defer wg.Done()
					if err := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
						for _, workloadID := range workloadIDs {
							ret := &types.RemoveWorkloadMessage{WorkloadID: workloadID, Success: true, Hook: []*bytes.Buffer{}}
							if err := c.withWorkloadLocked(ctx, workloadID, func(ctx context.Context, workload *types.Workload) error {
								return utils.Txn(
									ctx,
									// if
									func(ctx context.Context) error {
										_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []types.Resources{workload.Resources}, true, plugins.Decr)
										return err
									},
									// then
									func(ctx context.Context) (err error) {
										if err = c.doRemoveWorkload(ctx, workload, force); err == nil {
											logger.Infof(ctx, "Workload %s removed", workload.ID)
										}
										return err
									},
									// rollback
									func(ctx context.Context, failedByCond bool) error {
										if failedByCond {
											return nil
										}
										_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []types.Resources{workload.Resources}, true, plugins.Incr)
										return err
									},
									c.config.GlobalTimeout,
								)
							}); err != nil {
								logger.WithField("id", workloadID).Error(ctx, err, "failed to lock workload")
								ret.Hook = append(ret.Hook, bytes.NewBufferString(err.Error()))
								ret.Success = false
							}
							ch <- ret
						}
						_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })
						return nil
					}); err != nil {
						logger.WithField("node", nodename).Error(ctx, err, "failed to lock node")
						ch <- &types.RemoveWorkloadMessage{Success: false}
					}
				}
			}(nodename, workloadIDs))
		}
	})
	return ch, nil
}

// RemoveWorkloadSync .
func (c *Calcium) RemoveWorkloadSync(ctx context.Context, IDs []string) error {
	return c.doRemoveWorkloadSync(ctx, IDs)
}

// semantic: instance removed on err == nil, instance remained on err != nil
func (c *Calcium) doRemoveWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	return utils.Txn(
		ctx,
		// if
		func(ctx context.Context) error {
			return c.store.RemoveWorkload(ctx, workload)
		},
		// then
		func(ctx context.Context) error {
			return workload.Remove(ctx, force)
		},
		// rollback
		func(ctx context.Context, failedByCond bool) error {
			if failedByCond {
				return nil
			}
			return c.store.AddWorkload(ctx, workload, nil)
		},
		c.config.GlobalTimeout,
	)
}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *Calcium) doRemoveWorkloadSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveWorkload(ctx, IDs, true)
	if err != nil {
		return err
	}

	for m := range ch {
		// TODO deal with failed
		log.WithFunc("calcium.doRemoveWorkloadSync").Debugf(ctx, "Removed %s", m.WorkloadID)
	}
	return nil
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
