package calcium

import (
	"context"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// DissociateWorkload dissociate workload from eru, return it resource but not modity it
func (c *Calcium) DissociateWorkload(ctx context.Context, ids []string) (chan *types.DissociateWorkloadMessage, error) {
	logger := log.WithField("Caliucm", "Dissociate").WithField("ids", ids)
	ch := make(chan *types.DissociateWorkloadMessage)
	go func() {
		defer close(ch)
		// TODO@zc: group ids by node
		for _, id := range ids {
			err := c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
				return c.withNodeLocked(ctx, workload.Nodename, func(ctx context.Context, node *types.Node) (err error) {
					err = utils.Txn(
						ctx,
						// if
						func(ctx context.Context) error {
							log.Infof("[DissociateWorkload] Workload %s dissociated", workload.ID)
							return errors.WithStack(c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, store.ActionIncr))
						},
						// then
						func(ctx context.Context) error {
							return errors.WithStack(c.store.RemoveWorkload(ctx, workload))
						},
						// rollback
						func(ctx context.Context, _ bool) error {
							return errors.WithStack(c.store.UpdateNodeResource(ctx, node, &workload.ResourceMeta, store.ActionDecr))
						},
						c.config.GlobalTimeout,
					)

					c.doRemapResourceAndLog(ctx, logger, node)
					return err
				})
			})
			if err != nil {
				logger.Errorf("[DissociateWorkload] Dissociate workload %s failed, err: %+v", id, err)
			}
			ch <- &types.DissociateWorkloadMessage{WorkloadID: id, Error: err}
		}
	}()
	return ch, nil
}
