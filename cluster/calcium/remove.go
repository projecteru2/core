package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/utils"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// RemoveContainer remove containers
// returns a channel that contains removing responses
func (c *Calcium) RemoveContainer(ctx context.Context, IDs []string, force bool, step int) (chan *types.RemoveContainerMessage, error) {
	ch := make(chan *types.RemoveContainerMessage)
	if step < 1 {
		step = 1
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for i, ID := range IDs {
			wg.Add(1)
			go func(ID string) {
				defer wg.Done()
				ret := &types.RemoveContainerMessage{ContainerID: ID, Success: false, Hook: []*bytes.Buffer{}}
				if err := c.withContainerLocked(ctx, ID, func(container *types.Container) error {
					return c.withNodeLocked(ctx, container.Nodename, func(node *types.Node) (err error) {
						return utils.Txn(
							ctx,
							// if
							func(ctx context.Context) error {
								return c.doRemoveContainer(ctx, container, force)
							},
							// then
							func(ctx context.Context) error {
								log.Infof("[RemoveContainer] Container %s removed", container.ID)
								return c.store.UpdateNodeResource(ctx, node, &container.Resource, store.ActionIncr)
							},
							// rollback
							nil,
							c.config.GlobalTimeout,
						)
					})
				}); err != nil {
					log.Errorf("[RemoveContainer] Remove container %s failed, err: %v", ID, err)
					ret.Hook = append(ret.Hook, bytes.NewBufferString(err.Error()))
				} else {
					ret.Success = true
				}
				ch <- ret
			}(ID)
			if (i+1)%step == 0 {
				log.Info("[RemoveContainer] Wait for previous tasks done")
				wg.Wait()
			}
		}
	}()
	return ch, nil
}

func (c *Calcium) doRemoveContainer(ctx context.Context, container *types.Container, force bool) error {
	return utils.Txn(
		ctx,
		// if
		func(ctx context.Context) error {
			return container.Remove(ctx, force)
		},
		// then
		func(ctx context.Context) error {
			return c.store.RemoveContainer(ctx, container)
		},
		// rollback
		nil,
		c.config.GlobalTimeout,
	)

}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *Calcium) doRemoveContainerSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveContainer(ctx, IDs, true, 1)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[doRemoveContainerSync] Removed %s", m.ContainerID)
	}
	return nil
}
