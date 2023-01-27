package calcium

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type remapMsg struct {
	ID  string
	err error
}

// RemapResourceAndLog called on changes of resource binding, such as cpu binding
// as an internal api, remap doesn't lock node, the responsibility of that should be taken on by caller
func (c *Calcium) RemapResourceAndLog(ctx context.Context, logger *log.Fields, node *types.Node) {
	ctx, cancel := context.WithTimeout(utils.NewInheritCtx(ctx), c.config.GlobalTimeout)
	defer cancel()

	err := c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
		if ch, err := c.doRemapResource(ctx, node); err == nil {
			for msg := range ch {
				logger.Infof(ctx, "remap workload ID %+v", msg.ID)
				if msg.err != nil {
					logger.Error(ctx, msg.err)
				}
			}
		}
		return nil
	})

	if err != nil {
		logger.Error(ctx, err, "remap node failed")
	}
}

func (c *Calcium) doRemapResource(ctx context.Context, node *types.Node) (ch chan *remapMsg, err error) {
	workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
	if err != nil {
		return
	}

	engineParamsMap, err := c.rmgr2.Remap(ctx, node.Name, workloads)
	if err != nil {
		return nil, err
	}

	ch = make(chan *remapMsg, len(*engineParamsMap))
	_ = c.pool.Invoke(func() {
		defer close(ch)
		for workloadID, engineParams := range *engineParamsMap {
			ch <- &remapMsg{
				ID:  workloadID,
				err: node.Engine.VirtualizationUpdateResource(ctx, workloadID, &enginetypes.VirtualizationResource{EngineParams: engineParams}),
			}
		}
	})

	return ch, nil
}
