package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/store"

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
				output := []*bytes.Buffer{}
				success := false
				if err := c.withContainerLocked(ctx, ID, func(container *types.Container) error {
					return c.withNodeLocked(ctx, container.Nodename, func(node *types.Node) (err error) {
						return c.Transaction(
							ctx,
							// if
							func(ctx context.Context) error {
								return c.doRemoveContainer(ctx, container, force)
							},
							// then
							func(ctx context.Context) error {
								log.Infof("[RemoveContainer] Container %s removed", container.ID)
								return c.store.UpdateNodeResource(ctx, node, container.CPU, container.Quota, container.Memory, container.Storage, container.VolumePlan.IntoVolumeMap(), store.ActionIncr)
							},
							// rollback
							nil,
						)
					})
				}); err != nil {
					log.Errorf("[RemoveContainer] Remove container %s failed, err: %v", ID, err)
					output = append(output, bytes.NewBufferString(err.Error()))
				} else {
					success = true
				}
				ch <- &types.RemoveContainerMessage{ContainerID: ID, Success: success, Hook: output}
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
	return c.Transaction(
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
