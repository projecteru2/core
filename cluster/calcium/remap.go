package calcium

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type remapMsg struct {
	id  string
	err error
}

func (c *Calcium) doRemapResourceAndLog(ctx context.Context, logger *log.Fields, node *types.Node) {
	logger.Infof(ctx, "[doRemapResourceAndLog] remap node %s", node.Name)
	ctx, cancel := context.WithTimeout(utils.InheritTracingInfo(ctx, context.TODO()), c.config.GlobalTimeout)
	defer cancel()

	err := c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
		logger = logger.WithField("Calcium", "doRemapResourceAndLog").WithField("nodename", node.Name)
		if ch, err := c.remapResource(ctx, node); err == nil {
			for msg := range ch {
				logger.Infof(ctx, "[doRemapResourceAndLog] id %+v", msg.id)
				logger.WithField("id", msg.id).Error(ctx, msg.err) //nolint:errcheck
			}
		}
		return nil
	})

	if err != nil {
		logger.Errorf(ctx, err, "[doRemapResourceAndLog] remap node %s failed", node.Name)
	}
}

// called on changes of resource binding, such as cpu binding
// as an internal api, remap doesn't lock node, the responsibility of that should be taken on by caller
func (c *Calcium) remapResource(ctx context.Context, node *types.Node) (ch chan *remapMsg, err error) {
	workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
	if err != nil {
		return
	}

	workloadMap := map[string]*types.Workload{}
	for _, workload := range workloads {
		workloadMap[workload.ID] = workload
	}

	engineArgsMap, err := c.rmgr.GetRemapArgs(ctx, node.Name, workloadMap)
	if err != nil {
		return nil, err
	}

	ch = make(chan *remapMsg, len(engineArgsMap))
	_ = c.pool.Invoke(func() {
		defer close(ch)
		for workloadID, engineArgs := range engineArgsMap {
			ch <- &remapMsg{
				id:  workloadID,
				err: node.Engine.VirtualizationUpdateResource(ctx, workloadID, &enginetypes.VirtualizationResource{EngineArgs: engineArgs}),
			}
		}
	})

	return ch, nil
}
