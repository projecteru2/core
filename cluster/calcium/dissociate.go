package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) DissociateWorkload(ctx context.Context, IDs []string) (chan *types.DissociateWorkloadMessage, error) {
	logger := log.WithFunc("calcium.DissociateWorkload").WithField("IDs", IDs)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, IDs)
	if err != nil {
		logger.Error(ctx, err, "failed to group workloads by node")
		return nil, err
	}

	ch := make(chan *types.DissociateWorkloadMessage)
	caller := ctx
	utils.SentryGo(func() {
		defer close(ch)

		for nodename, workloadIDs := range nodeWorkloadGroup {
			if nodeErr := c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
				for _, workloadID := range workloadIDs {
					msg := &types.DissociateWorkloadMessage{WorkloadID: workloadID}
					if workloadErr := c.withWorkloadLocked(ctx, workloadID, false, func(ctx context.Context, workload *types.Workload) error {
						return c.withResourceReleased(ctx, node, workload, func(ctx context.Context) error {
							return c.store.RemoveWorkload(ctx, workload)
						})
					}); workloadErr != nil {
						logger.WithField("id", workloadID).Error(ctx, workloadErr, "failed to dissociate workload")
						msg.Error = workloadErr
					}
					if err := send(caller, ch, msg); err != nil {
						return err
					}
				}
				c.invokePoolAsync(func() { c.RemapResourceAndLog(ctx, logger, node) })
				return nil
			}); nodeErr != nil {
				logger.WithField("node", nodename).Error(ctx, nodeErr, "failed to lock node")
			}
		}
	})
	return ch, nil
}
