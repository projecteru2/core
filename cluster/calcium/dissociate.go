package calcium

import (
	"context"

	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// DissociateContainer dissociate container from eru, return it resource but not modity it
func (c *Calcium) DissociateContainer(ctx context.Context, IDs []string) (chan *types.DissociateContainerMessage, error) {
	ch := make(chan *types.DissociateContainerMessage)
	go func() {
		defer close(ch)
		for _, ID := range IDs {
			err := c.withContainerLocked(ctx, ID, func(container *types.Container) error {
				return c.withNodeLocked(ctx, container.Nodename, func(node *types.Node) (err error) {
					return c.Transaction(
						ctx,
						// if
						func(ctx context.Context) error {
							return c.store.RemoveContainer(ctx, container)
						},
						// then
						func(ctx context.Context) error {
							log.Infof("[DissociateContainer] Container %s dissociated", container.ID)
							return c.store.UpdateNodeResource(ctx, node, container.CPU, container.Quota, container.Memory, container.Storage, container.VolumePlan.IntoVolumeMap(), store.ActionIncr)
						},
						// rollback
						nil,
					)
				})
			})
			if err != nil {
				log.Errorf("[DissociateContainer] Dissociate container %s failed, err: %v", ID, err)
			}
			ch <- &types.DissociateContainerMessage{ContainerID: ID, Error: err}
		}
	}()
	return ch, nil
}
