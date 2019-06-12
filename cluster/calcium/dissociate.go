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
			success := false
			if err := c.withContainerLocked(ctx, ID, func(container *types.Container) error {
				return c.withNodeLocked(ctx, container.Podname, container.Nodename, func(node *types.Node) (err error) {
					if err := c.store.RemoveContainer(ctx, container); err != nil {
						return err
					}
					log.Infof("[DissociateContainer] Container %s dissociated", container.ID)
					return c.store.UpdateNodeResource(ctx, node, container.CPU, container.Quota, container.Memory, store.ActionIncr)
				})
			}); err != nil {
				log.Errorf("[DissociateContainer] Dissociate container %s failed, err: %v", ID, err)
			} else {
				success = true
			}
			ch <- &types.DissociateContainerMessage{ContainerID: ID, Success: success}
		}
	}()
	return ch, nil
}
