package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/utils"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// RemoveWorkload remove workloads
// returns a channel that contains removing responses
func (c *Calcium) RemoveWorkload(ctx context.Context, ids []string, force bool, step int) (chan *types.RemoveWorkloadMessage, error) {
	logger := log.WithField("Calcium", "RemoveWorkload").WithField("ids", ids).WithField("force", force).WithField("step", step)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, ids)
	if err != nil {
		logger.Errorf("failed to group workloads by node: %+v", err)
		return nil, errors.WithStack(err)
	}

	ch := make(chan *types.RemoveWorkloadMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for nodename, workloadIDs := range nodeWorkloadGroup {
			wg.Add(1)
			go func(nodename string, workloadIDs []string) {
				defer wg.Done()
				if err := c.withNodeLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
					for _, workloadID := range workloadIDs {
						if err := c.withWorkloadLocked(ctx, workloadID, func(ctx context.Context, workload *types.Workload) error {
							ret := &types.RemoveWorkloadMessage{WorkloadID: workloadID, Success: true, Hook: []*bytes.Buffer{}}
							if err := utils.Txn(
								ctx,
								// if
								func(ctx context.Context) error {
									return errors.WithStack(c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, store.ActionIncr))
								},
								// then
								func(ctx context.Context) error {
									err := errors.WithStack(c.doRemoveWorkload(ctx, workload, force))
									if err != nil {
										log.Infof("[RemoveWorkload] Workload %s removed", workload.ID)
									}
									return err
								},
								// rollback
								func(ctx context.Context, failedByCond bool) error {
									if failedByCond {
										return nil
									}
									return errors.WithStack(c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, store.ActionDecr))
								},
								c.config.GlobalTimeout,
							); err != nil {
								logger.WithField("id", workloadID).Errorf("[RemoveWorkload] Remove workload failed: %+v", err)
								ret.Hook = append(ret.Hook, bytes.NewBufferString(err.Error()))
								ret.Success = false
							}

							ch <- ret
							return nil
						}); err != nil {
							logger.WithField("id", workloadID).Errorf("failed to lock workload: %+v", err)
						}
					}
					c.doRemapResourceAndLog(ctx, logger, node)
					return nil
				}); err != nil {
					logger.WithField("nodename", nodename).Errorf("failed to lock node: %+v", err)
				}
			}(nodename, workloadIDs)
		}
	}()
	return ch, nil
}

// semantic: instance removed on err == nil, instance remained on err != nil
func (c *Calcium) doRemoveWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	return utils.Txn(
		ctx,
		// if
		func(ctx context.Context) error {
			return errors.WithStack(c.store.RemoveWorkload(ctx, workload))
		},
		// then
		func(ctx context.Context) error {
			return errors.WithStack(workload.Remove(ctx, force))
		},
		// rollback
		func(ctx context.Context, failedByCond bool) error {
			if failedByCond {
				return nil
			}
			return errors.WithStack(c.store.AddWorkload(ctx, workload))
		},
		c.config.GlobalTimeout,
	)

}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *Calcium) doRemoveWorkloadSync(ctx context.Context, ids []string) error {
	ch, err := c.RemoveWorkload(ctx, ids, true, 1)
	if err != nil {
		return errors.WithStack(err)
	}

	for m := range ch {
		log.Debugf("[doRemoveWorkloadSync] Removed %s", m.WorkloadID)
	}
	return nil
}

func (c *Calcium) groupWorkloadsByNode(ctx context.Context, ids []string) (map[string][]string, error) {
	workloads, err := c.store.GetWorkloads(ctx, ids)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	nodeWorkloadGroup := map[string][]string{}
	for _, workload := range workloads {
		nodeWorkloadGroup[workload.Nodename] = append(nodeWorkloadGroup[workload.Nodename], workload.ID)
	}
	return nodeWorkloadGroup, nil
}
