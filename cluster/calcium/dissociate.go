package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// DissociateWorkload dissociate workload from eru, return it resource but not modity it
func (c *Calcium) DissociateWorkload(ctx context.Context, ids []string) (chan *types.DissociateWorkloadMessage, error) {
	ch := make(chan *types.DissociateWorkloadMessage)
	go func() {
		defer close(ch)
		for _, id := range ids {
			err := c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
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
				log.Errorf("[DissociateWorkload] Dissociate workload %s failed, err: %v", id, err)
			}
			ch <- &types.DissociateWorkloadMessage{WorkloadID: id, Error: err}
		}
	}()
	return ch, nil
}
