package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/cluster"
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
		ib := newImageBucket()

		for _, ID := range IDs {
			// 单个单个取是因为某些情况可能会传了id但是这里没有
			// 这种情况不希望直接打断操作, 而是希望错误在message里回去.
			container, err := c.GetContainer(ctx, ID)
			if err != nil {
				ch <- &types.RemoveContainerMessage{
					ContainerID: ID,
					Success:     false,
					Message:     err.Error(),
				}
				continue
			}

			wg.Add(1)
			go func(container *types.Container) {
				defer wg.Done()

				message, err := c.doStopAndRemoveContainer(ctx, container, ib, force)
				success := false

				defer func() {
					ch <- &types.RemoveContainerMessage{
						ContainerID: container.ID,
						Success:     success,
						Message:     message,
					}
				}()

				if err != nil {
					return
				}

				log.Debugf("[RemoveContainer] Restore node %s resource cpu: %v mem: %v", container.Nodename, container.CPU, container.Memory)
				if err = c.store.UpdateNodeResource(ctx, container.Podname, container.Nodename, container.CPU, container.Memory, "+"); err != nil {
					log.Errorf("[RemoveContainer] Update Node resource failed %v", err)
					return
				}

				success = true
			}(container)
		}
		wg.Wait()

		// 把收集的image清理掉
		//TODO 如果 remove 是异步的，这里就不能用 ctx 了，gRPC 一断这里就会死
		go c.cleanCachedImage(ctx, ib)
	}()
	return ch, nil
}

func (c *Calcium) doStopAndRemoveContainer(ctx context.Context, container *types.Container, ib *imageBucket, force bool) (string, error) {
	lock, err := c.Lock(ctx, fmt.Sprintf(cluster.ContainerLock, container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return err.Error(), err
	}
	defer lock.Unlock(ctx)

	// 确保是有这个容器的
	containerJSON, err := container.Inspect(ctx)
	if err != nil {
		return err.Error(), err
	}

	var message string
	message, err = c.doStopContainer(ctx, container, containerJSON, ib, force)
	if err != nil {
		return message, err
	}

	if err = c.doRemoveContainer(ctx, container); err != nil {
		message += err.Error()
	}
	return message, err
}

func (c *Calcium) cleanCachedImage(ctx context.Context, ib *imageBucket) {
	for podname, images := range ib.Dump() {
		log.Debugf("[cleanCachedImage] clean %s images %v", podname, images)
		for _, image := range images {
			err := c.cleanImage(ctx, podname, image)
			if err != nil {
				log.Errorf("[doCleanImage] clean image failed %v", err)
			}
		}
	}
}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *Calcium) removeContainerSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveContainer(ctx, IDs, true)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[removeContainerSync] Removed %s", m.ContainerID)
	}
	return nil
}
