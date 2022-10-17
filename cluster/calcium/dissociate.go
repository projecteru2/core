package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// DissociateWorkload dissociate workload from eru, return it resource but not modity it
func (c *Calcium) DissociateWorkload(ctx context.Context, ids []string) (chan *types.DissociateWorkloadMessage, error) {
	logger := log.WithField("Caliucm", "Dissociate").WithField("ids", ids)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, ids)
	if err != nil {
		logger.Errorf(ctx, err, "failed to group workloads by node: %+v", err)
		return nil, err
	}

	ch := make(chan *types.DissociateWorkloadMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)

		for nodename, workloadIDs := range nodeWorkloadGroup {
			if err := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
				for _, workloadID := range workloadIDs { // nolint:scopelint
					msg := &types.DissociateWorkloadMessage{WorkloadID: workloadID} // nolint:scopelint
					if err := c.withWorkloadLocked(ctx, workloadID, func(ctx context.Context, workload *types.Workload) error {
						return utils.Txn(
							ctx,
							// if
							func(ctx context.Context) (err error) {
								resourceArgs := map[string]types.WorkloadResourceArgs{}
								for plugin, args := range workload.ResourceArgs {
									resourceArgs[plugin] = args
								}
								_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []map[string]types.WorkloadResourceArgs{resourceArgs}, true, resources.Decr)
								return err
							},
							// then
							func(ctx context.Context) error {
								return c.store.RemoveWorkload(ctx, workload)
							},
							// rollback
							func(ctx context.Context, failedByCond bool) error {
								if failedByCond {
									return nil
								}
								resourceArgs := map[string]types.WorkloadResourceArgs{}
								for plugin, args := range workload.ResourceArgs {
									resourceArgs[plugin] = args
								}
								_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []map[string]types.WorkloadResourceArgs{resourceArgs}, true, resources.Incr)
								return err
							},
							c.config.GlobalTimeout,
						)
					}); err != nil {
						logger.WithField("id", workloadID).Errorf(ctx, err, "failed to lock workload: %+v", err)
						msg.Error = err
					}
					ch <- msg
				}
				_ = c.pool.Invoke(func() { c.doRemapResourceAndLog(ctx, logger, node) })
				return nil
			}); err != nil {
				logger.WithField("nodename", nodename).Errorf(ctx, err, "failed to lock node: %+v", err)
			}
		}
	})
	return ch, nil
}
