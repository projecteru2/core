package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/store"

	enginetypes "github.com/docker/docker/api/types"
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
			container, containerJSON, containerLock, err := c.LockAndGetContainer(ctx, ID)
			if err != nil {
				ch <- &types.RemoveContainerMessage{
					ContainerID: ID,
					Success:     false,
					Message:     err.Error(),
				}
				continue
			}
			wg.Add(1)
			go func(container *types.Container, containerJSON enginetypes.ContainerJSON, containerLock lock.DistributedLock) {
				defer wg.Done()
				// force to unlock
				defer containerLock.Unlock(context.Background())
				message := ""
				success := false

				defer func() {
					ch <- &types.RemoveContainerMessage{
						ContainerID: container.ID,
						Success:     success,
						Message:     message,
					}
				}()

				node, nodeLock, err := c.LockAndGetNode(ctx, container.Podname, container.Nodename)
				if err != nil {
					return
				}
				defer nodeLock.Unlock(context.Background())

				message, err = c.stopAndRemoveContainer(ctx, container, containerJSON, ib, force)
				if err != nil {
					return
				}

				log.Infof("[RemoveContainer] Container %s removed", container.ID)
				if err = c.store.UpdateNodeResource(ctx, node, container.CPU, container.Memory, store.ActionIncr); err != nil {
					log.Errorf("[RemoveContainer] Container %s removed, but update Node resource failed %v", container.ID, err)
					return
				}

				success = true
			}(container, containerJSON, containerLock)
		}
		wg.Wait()

		// 把收集的image清理掉
		// 如果 remove 是异步的，这里就不能用 ctx 了，gRPC 一断这里就会死
		go c.cleanCachedImage(context.Background(), ib)
	}()
	return ch, nil
}

func (c *Calcium) stopAndRemoveContainer(ctx context.Context, container *types.Container, containerJSON enginetypes.ContainerJSON, ib *imageBucket, force bool) (string, error) {
	message, err := c.doStopContainer(ctx, container, containerJSON, ib, force)
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
