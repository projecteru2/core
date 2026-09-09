package calcium

import (
	"context"
	"maps"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.DeployOptions) (*types.CapacityMessage, error) {
	logger := log.WithFunc("calcium.CalculateCapacity").WithField("opts", opts)
	logger.Infof(ctx, "calculate capacity with options:\n%s", litter.Options{Compact: true}.Sdump(opts))
	msg := &types.CapacityMessage{
		Total:          0,
		NodeCapacities: map[string]int{},
	}

	return msg, c.withNodes(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) error {
		nodenames := slices.Collect(maps.Keys(nodeMap))

		if opts.DeployStrategy != strategy.Dummy {
			capacities, err := c.doGetDeployStrategy(ctx, nodenames, opts)
			if err != nil {
				logger.Error(ctx, err, "failed to get deploy strategy")
				return err
			}
			msg.NodeCapacities = capacities

			for _, capacity := range capacities {
				msg.Total += capacity
			}
			return nil
		}

		infos, total, err := c.rmgr.GetNodesDeployCapacity(ctx, nodenames, opts.Resources)
		if err != nil {
			logger.Error(ctx, err, "failed to get nodes capacity")
			return err
		}
		msg.Total = total
		if msg.Total <= 0 {
			return errors.Wrap(types.ErrInsufficientResource, "no node meets all the resource requirements at the same time")
		}
		for node, info := range infos {
			msg.NodeCapacities[node] = info.Capacity
		}
		return nil
	})
}
