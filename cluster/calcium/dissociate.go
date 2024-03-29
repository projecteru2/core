package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// DissociateWorkload dissociate workload from eru, return it resource but not modity it
func (c *Calcium) DissociateWorkload(ctx context.Context, IDs []string) (chan *types.DissociateWorkloadMessage, error) {
	logger := log.WithFunc("caliucm.DissociateWorkload").WithField("IDs", IDs)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, IDs)
	if err != nil {
		logger.Error(ctx, err, "failed to group workloads by node")
		return nil, err
	}

	ch := make(chan *types.DissociateWorkloadMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)

		for nodename, workloadIDs := range nodeWorkloadGroup {
			if err := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
				for _, workloadID := range workloadIDs { //nolint:scopelint
					msg := &types.DissociateWorkloadMessage{WorkloadID: workloadID} //nolint:scopelint
					if err := c.withWorkloadLocked(ctx, workloadID, false, func(ctx context.Context, workload *types.Workload) error {
						return utils.Txn(
							ctx,
							// if
							func(ctx context.Context) (err error) {
								_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []resourcetypes.Resources{workload.Resources}, true, plugins.Decr)
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
								_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []resourcetypes.Resources{workload.Resources}, true, plugins.Incr)
								return err
							},
							c.config.GlobalTimeout,
						)
					}); err != nil {
						logger.WithField("id", workloadID).Error(ctx, err, "failed to lock workload")
						msg.Error = err
					}
					ch <- msg
				}
				_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })
				return nil
			}); err != nil {
				logger.WithField("node", nodename).Error(ctx, err, "failed to lock node")
			}
		}
	})
	return ch, nil
}
