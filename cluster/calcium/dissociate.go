package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) DissociateWorkload(ctx context.Context, IDs []string) (chan *types.DissociateWorkloadMessage, error) {
	logger := log.WithFunc("calcium.DissociateWorkload").WithField("IDs", IDs)
	ch := make(chan *types.DissociateWorkloadMessage)
	err := c.releaseWorkloads(ctx, logger, IDs, func(ctx context.Context, _ *types.Node, workload *types.Workload) error {
		return c.store.RemoveWorkload(ctx, workload)
	}, func(workloadID string, err error) error {
		if err != nil {
			logger.WithField("id", workloadID).Error(ctx, err, "failed to dissociate workload")
		}
		return send(ctx, ch, &types.DissociateWorkloadMessage{WorkloadID: workloadID, Error: err})
	}, func() { close(ch) })
	if err != nil {
		logger.Error(ctx, err, "failed to group workloads by node")
		return nil, err
	}
	return ch, nil
}
