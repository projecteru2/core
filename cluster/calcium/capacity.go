package calcium

import (
	"context"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

// CalculateCapacity calculates capacity
func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.DeployOptions) (*types.CapacityMessage, error) {
	var err error
	msg := &types.CapacityMessage{
		Total:          0,
		NodeCapacities: map[string]int{},
	}
	return msg, c.withNodesLocked(ctx, opts.Podname, opts.Nodenames, nil, false, func(ctx context.Context, nodeMap map[string]*types.Node) error {
		if opts.DeployStrategy != strategy.Dummy {
			if _, msg.NodeCapacities, err = c.doAllocResource(ctx, nodeMap, opts); err != nil {
				return errors.WithStack(err)
			}

			for _, capacity := range msg.NodeCapacities {
				msg.Total += capacity
			}
		} else {
			var infos []strategy.Info
			msg.Total, _, infos, err = c.doCalculateCapacity(nodeMap, opts)
			if err != nil {
				return errors.WithStack(err)
			}
			for _, info := range infos {
				msg.NodeCapacities[info.Nodename] = info.Capacity
			}
		}
		return nil
	})
}

func (c *Calcium) doCalculateCapacity(nodeMap map[string]*types.Node, opts *types.DeployOptions) (
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
		return 0, nil, nil, errors.WithStack(err)
	}

	// select available nodes
	if plans, err = resources.SelectNodesByResourceRequests(resourceRequests, nodeMap); err != nil {
		return 0, nil, nil, errors.WithStack(err)
	}
	log.Debugf("[Calcium.doCalculateCapacity] plans: %+v, total: %v", plans, total)

	// deploy strategy
	infos = strategy.NewInfos(resourceRequests, nodeMap, plans)
	for _, info := range infos {
		total += info.Capacity
	}
	return
}
