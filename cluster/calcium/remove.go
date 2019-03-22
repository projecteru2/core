package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/store"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// RemoveContainer remove containers
// returns a channel that contains removing responses
func (c *Calcium) RemoveContainer(ctx context.Context, IDs []string, force bool) (chan *types.RemoveContainerMessage, error) {
	ch := make(chan *types.RemoveContainerMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		for _, ID := range IDs {
			container, containerJSON, containerLock, err := c.doLockAndGetContainer(ctx, ID)
			if err != nil {
				ch <- &types.RemoveContainerMessage{
					ContainerID: ID,
					Success:     false,
					Hook:        []*bytes.Buffer{bytes.NewBufferString(err.Error())},
				}
				continue
			}
			wg.Add(1)
			go func(container *types.Container, containerJSON *enginetypes.VirtualizationInfo, containerLock lock.DistributedLock) {
				defer wg.Done()
				// force to unlock
				defer c.doUnlock(containerLock, container.ID)
				output := []*bytes.Buffer{}
				success := false

				defer func() {
					ch <- &types.RemoveContainerMessage{
						ContainerID: container.ID,
						Success:     success,
						Hook:        output,
					}
				}()

				node, nodeLock, err := c.doLockAndGetNode(ctx, container.Podname, container.Nodename)
				if err != nil {
					return
				}
				defer c.doUnlock(nodeLock, node.Name)

				output, err = c.doStopAndRemoveContainer(ctx, container, containerJSON, force)
				if err != nil {
					return
				}

				log.Infof("[RemoveContainer] Container %s removed", container.ID)
				if err = c.store.UpdateNodeResource(ctx, node, container.CPU, container.Quota, container.Memory, store.ActionIncr); err != nil {
					log.Errorf("[RemoveContainer] Container %s removed, but update Node resource failed %v", container.ID, err)
					return
				}

				success = true
			}(container, containerJSON, containerLock)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (c *Calcium) doStopAndRemoveContainer(ctx context.Context, container *types.Container, containerJSON *enginetypes.VirtualizationInfo, force bool) ([]*bytes.Buffer, error) {
	message, err := c.doStopContainer(ctx, container, containerJSON, force)
	if err != nil {
		return message, err
	}

	if err = c.doRemoveContainer(ctx, container); err != nil {
		message = append(message, bytes.NewBufferString(err.Error()))
	}
	return message, err
}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *Calcium) doRemoveContainerSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveContainer(ctx, IDs, true)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[doRemoveContainerSync] Removed %s", m.ContainerID)
	}
	return nil
}

func (c *Calcium) doRemoveContainer(ctx context.Context, container *types.Container) error {
	if err := container.Remove(ctx); err != nil {
		return err
	}

	return c.store.RemoveContainer(ctx, container)
}
