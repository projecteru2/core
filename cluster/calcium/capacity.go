package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/sanity-io/litter"

	"github.com/pkg/errors"
)

// CalculateCapacity calculates capacity
func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.DeployOptions) (*types.CapacityMessage, error) {
	logger := log.WithField("Calcium", "CalculateCapacity").WithField("opts", opts)
	var err error
	log.Infof(ctx, "[CalculateCapacity] Calculate capacity with options:\n%s", litter.Options{Compact: true}.Sdump(opts))
	msg := &types.CapacityMessage{
		Total:          0,
		NodeCapacities: map[string]int{},
	}

	return msg, c.withNodesPodLocked(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) error {
		nodes := []string{}
		for node := range nodeMap {
			nodes = append(nodes, node)
		}

		if opts.DeployStrategy != strategy.Dummy {
			if msg.NodeCapacities, err = c.doGetDeployMap(ctx, nodes, opts); err != nil {
				logger.Errorf(ctx, "[Calcium.CalculateCapacity] doGetDeployMap failed: %+v", err)
				return err
			}

			for _, capacity := range msg.NodeCapacities {
				msg.Total += capacity
			}
		} else {
			var infos map[string]*resources.NodeCapacityInfo
			infos, msg.Total, err = c.rmgr.GetNodesDeployCapacity(ctx, nodes, opts.ResourceOpts)
			if err != nil {
				logger.Errorf(ctx, "[Calcium.CalculateCapacity] failed to get nodes capacity: %+v", err)
				return err
			}
			if msg.Total <= 0 {
				return errors.Wrap(types.ErrInsufficientRes, "no node meets all the resource requirements at the same time")
			}
			for node, info := range infos {
				msg.NodeCapacities[node] = info.Capacity
			}
		}
		return nil
	})
}
