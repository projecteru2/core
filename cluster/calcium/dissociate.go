package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// DissociateWorkload dissociate workload from eru, return it resource but not modity it
func (c *Calcium) DissociateWorkload(ctx context.Context, IDs []string) (chan *types.DissociateWorkloadMessage, error) {
	ch := make(chan *types.DissociateWorkloadMessage)
	go func() {
		defer close(ch)
		for _, ID := range IDs {
			err := c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
				return c.withNodeLocked(ctx, workload.Nodename, func(ctx context.Context, node *types.Node) (err error) {
					return utils.Txn(
						ctx,
						// if
						func(ctx context.Context) error {
							return c.store.RemoveWorkload(ctx, workload)
						},
						// then
						func(ctx context.Context) error {
							log.Infof("[DissociateWorkload] Workload %s dissociated", workload.ID)
							return c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, store.ActionIncr)
						},
						// rollback
						nil,
						c.config.GlobalTimeout,
					)
				})
			})
			if err != nil {
				log.Errorf("[DissociateWorkload] Dissociate workload %s failed, err: %v", ID, err)
			}
			ch <- &types.DissociateWorkloadMessage{WorkloadID: ID, Error: err}
		}
	}()
	return ch, nil
}
