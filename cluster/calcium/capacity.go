package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/sanity-io/litter"
	"golang.org/x/exp/maps"

	"github.com/cockroachdb/errors"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
)

// CalculateCapacity calculates capacity
func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.DeployOptions) (*types.CapacityMessage, error) {
	logger := log.WithFunc("calcium.CalculateCapacity").WithField("opts", opts)
	logger.Infof(ctx, "Calculate capacity with options:\n%s", litter.Options{Compact: true}.Sdump(opts))
	var err error
	msg := &types.CapacityMessage{
		Total:          0,
		NodeCapacities: map[string]int{},
	}

	return msg, c.withNodesPodLocked(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) error {
		nodenames := maps.Keys(nodeMap)

		if opts.DeployStrategy != strategy.Dummy {
			if msg.NodeCapacities, err = c.doGetDeployStrategy(ctx, nodenames, opts); err != nil {
				logger.Error(ctx, err, "doGetDeployMap failed")
				return err
			}

			for _, capacity := range msg.NodeCapacities {
				msg.Total += capacity
			}
			return nil
		}

		var infos map[string]*plugintypes.NodeDeployCapacity
		infos, msg.Total, err = c.rmgr2.GetNodesDeployCapacity(ctx, nodenames, opts.Resources)
		if err != nil {
			logger.Error(ctx, err, "failed to get nodes capacity")
			return err
		}
		if msg.Total <= 0 {
			return errors.Wrap(types.ErrInsufficientResource, "no node meets all the resource requirements at the same time")
		}
		for node, info := range infos {
			msg.NodeCapacities[node] = info.Capacity
		}
		return nil
	})
}
