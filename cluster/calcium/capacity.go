package calcium

import (
	"context"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/types"
)

// CalculateCapacity calculates capacity
func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.DeployOptions) (msg *types.CapacityMessage, err error) {
	return msg, c.withNodesLocked(ctx, opts.Podname, opts.Nodenames, nil, true, func(nodeMap map[string]*types.Node) error {
		var deployMap map[string]int
		if _, deployMap, err = c.doAllocResource(ctx, nodeMap, opts); err != nil {
			return errors.WithStack(err)
		}

		msg = &types.CapacityMessage{NodeCapacities: deployMap}
		for _, capacity := range deployMap {
			msg.Total += capacity
		}
		return nil
	})
}
