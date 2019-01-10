package calcium

import (
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
		ib := newImageBucket()
		for _, ID := range IDs {
			container, containerJSON, containerLock, err := c.doLockAndGetContainer(ctx, ID)
			if err != nil {
				ch <- &types.RemoveContainerMessage{
					ContainerID: ID,
					Success:     false,
					Message:     err.Error(),
				}
				continue
			}
			wg.Add(1)
			go func(container *types.Container, containerJSON *enginetypes.VirtualizationInfo, containerLock lock.DistributedLock) {
				defer wg.Done()
				// force to unlock
				defer c.doUnlock(containerLock, container.ID)
				message := ""
				success := false

				defer func() {
					ch <- &types.RemoveContainerMessage{
						ContainerID: container.ID,
						Success:     success,
						Message:     message,
					}
				}()

				node, nodeLock, err := c.doLockAndGetNode(ctx, container.Podname, container.Nodename)
				if err != nil {
					return
				}
				defer c.doUnlock(nodeLock, node.Name)

				message, err = c.doStopAndRemoveContainer(ctx, container, containerJSON, ib, force)
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

		// 把收集的image清理掉
		// 如果 remove 是异步的，这里就不能用 ctx 了，gRPC 一断这里就会死
		go c.doCleanCachedImage(context.Background(), ib)
	}()
	return ch, nil
}

func (c *Calcium) doStopAndRemoveContainer(ctx context.Context, container *types.Container, containerJSON *enginetypes.VirtualizationInfo, ib *imageBucket, force bool) (string, error) {
	message, err := c.doStopContainer(ctx, container, containerJSON, ib, force)
	if err != nil {
		return message, err
	}

	if err = c.doRemoveContainer(ctx, container); err != nil {
		message += err.Error()
	}
	return message, err
}

func (c *Calcium) doCleanCachedImage(ctx context.Context, ib *imageBucket) {
	for podname, images := range ib.Dump() {
		log.Debugf("[doCleanCachedImage] Clean %s images %v", podname, images)
		for _, image := range images {
			err := c.doCleanImage(ctx, podname, image)
			if err != nil {
				log.Errorf("[doCleanCachedImage] Clean image failed %v", err)
			}
		}
	}
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
