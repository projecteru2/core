package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// DissociateWorkload dissociate workload from eru, return it resource but not modity it
func (c *Calcium) DissociateWorkload(ctx context.Context, ids []string) (chan *types.DissociateWorkloadMessage, error) {
	logger := log.WithField("Caliucm", "Dissociate").WithField("ids", ids)

	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, ids)
	if err != nil {
		logger.Errorf(ctx, "failed to group workloads by node: %+v", err)
		return nil, err
	}

	ch := make(chan *types.DissociateWorkloadMessage)
	utils.SentryGo(func() {
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
								if err = c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, types.ActionIncr); err == nil {
									log.Infof(ctx, "[DissociateWorkload] Workload %s dissociated", workload.ID)
								}
								return errors.WithStack(err)
							},
							// then
							func(ctx context.Context) error {
								return errors.WithStack(c.store.RemoveWorkload(ctx, workload))
							},
							// rollback
							func(ctx context.Context, failedByCond bool) error {
								if failedByCond {
									return nil
								}
								return errors.WithStack(c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, types.ActionDecr))
							},
							c.config.GlobalTimeout,
						)
					}); err != nil {
						logger.WithField("id", workloadID).Errorf(ctx, "failed to lock workload: %+v", err)
						msg.Error = err
					}
					ch <- msg
				}
				go c.doRemapResourceAndLog(ctx, logger, node)
				return nil
			}); err != nil {
				logger.WithField("nodename", nodename).Errorf(ctx, "failed to lock node: %+v", err)
			}
		}
	})
	return ch, nil
}
