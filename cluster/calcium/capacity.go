package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
)

// CalculateCapacity calculates capacity
func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.DeployOptions) (*types.CapacityMessage, error) {
	logger := log.WithField("Calcium", "CalculateCapacity").WithField("opts", opts)
	var err error
	msg := &types.CapacityMessage{
		Total:          0,
		NodeCapacities: map[string]int{},
	}

	return msg, c.withNodesPodLocked(ctx, opts.NodeFilter, func(ctx context.Context, nodeMap map[string]*types.Node) error {
		if opts.DeployStrategy != strategy.Dummy {
			if _, msg.NodeCapacities, err = c.doAllocResource(ctx, nodeMap, opts); err != nil {
				logger.Errorf(ctx, "[Calcium.CalculateCapacity] doAllocResource failed: %+v", err)
				return err
			}

			for _, capacity := range msg.NodeCapacities {
				msg.Total += capacity
			}
		} else {
			var infos []strategy.Info
			msg.Total, _, infos, err = c.doCalculateCapacity(ctx, nodeMap, opts)
			if err != nil {
				logger.Errorf(ctx, "[Calcium.CalculateCapacity] doCalculateCapacity failed: %+v", err)
				return err
			}
			for _, info := range infos {
				msg.NodeCapacities[info.Nodename] = info.Capacity
			}
		}
		return nil
	})
}

func (c *Calcium) doCalculateCapacity(ctx context.Context, nodeMap map[string]*types.Node, opts *types.DeployOptions) (
	total int,
	plans []resourcetypes.ResourcePlans,
	infos []strategy.Info,
	err error,
) {
	if len(nodeMap) == 0 {
		return 0, nil, nil, errors.WithStack(types.ErrInsufficientNodes)
	}

	resourceRequests, err := resources.MakeRequests(opts.ResourceOpts)
	if err != nil {
		return 0, nil, nil, err
	}

	// select available nodes
	if plans, err = resources.SelectNodesByResourceRequests(ctx, resourceRequests, nodeMap); err != nil {
		return 0, nil, nil, err
	}

	// deploy strategy
	infos = strategy.NewInfos(resourceRequests, nodeMap, plans)
	for _, info := range infos {
		total += info.Capacity
	}
	log.Debugf(ctx, "[Calcium.doCalculateCapacity] plans: %+v, total: %v", plans, total)
	if total <= 0 {
		return 0, nil, nil, errors.Wrap(types.ErrInsufficientRes, "no node meets all the resource requirements at the same time")
	}
	return
}
