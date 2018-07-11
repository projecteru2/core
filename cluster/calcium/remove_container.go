package calcium

import (
	"context"
	"fmt"
	"strings"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

//RemoveContainer remove containers
// returns a channel that contains removing responses
func (c *Calcium) RemoveContainer(ctx context.Context, IDs []string, force bool) (chan *types.RemoveContainerMessage, error) {
	ch := make(chan *types.RemoveContainerMessage)
	go func() {
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

			info, err := container.Inspect(ctx)
			message := ""
			if err != nil {
				message = err.Error()
			} else {
				ib.Add(container.Podname, info.Config.Image)

			}

			wg.Add(1)
			go func(container *types.Container, info enginetypes.ContainerJSON, message string) {
				defer wg.Done()

				success := true

				defer func() {
					ch <- &types.RemoveContainerMessage{
						ContainerID: container.ID,
						Success:     success,
						Message:     message,
					}
				}()

				if container.Hook != nil && len(container.Hook.BeforeStop) > 0 && info.Config != nil {
					outputs := []string{}
					for _, cmd := range container.Hook.BeforeStop {
						output, err := execuateInside(ctx, container.Engine, container.ID, cmd, info.Config.User, info.Config.Env, container.Privileged)
						if err != nil {
							if container.Hook.Force && !force {
								success = false
								message = err.Error()
								return
							}
							outputs = append(outputs, err.Error())
							continue
						}
						outputs = append(outputs, string(output))
					}
					message = strings.Join(outputs, "")
				}

				if err := c.removeOneContainer(ctx, container); err != nil {
					success = false
					message += err.Error()
				}

			}(container, info, message)
		}

		wg.Wait()

		// 把收集的image清理掉
		//TODO 如果 remove 是异步的，这里就不能用 ctx 了，gRPC 一断这里就会死
		go func(ib *imageBucket) {
			for podname, images := range ib.Dump() {
				for _, image := range images {
					err := c.cleanImage(ctx, podname, image)
					if err != nil {
						log.Errorf("[RemoveContainer] clean image failed %v", err)
					}
				}
			}
		}(ib)
		close(ch)
	}()

	return ch, nil

}

// remove one container
func (c *Calcium) removeOneContainer(ctx context.Context, container *types.Container) error {
	var err error
	defer func() {
		if err != nil {
			log.Errorf("[removeOneContainer] Remove container failed, we have to check it manually %v", err)
			return
		}

		log.Debugf("[removeOneContainer] Restore node %s resource cpu: %v mem: %v", container.Nodename, container.CPU, container.Memory)
		if err := c.store.UpdateNodeResource(ctx, container.Podname, container.Nodename, container.CPU, container.Memory, "+"); err != nil {
			log.Errorf("[removeOneContainer] Update Node resource failed %v", err)
		}
	}()

	// 没 ID 就只做回收
	if container.ID == "" {
		return nil
	}

	// use etcd lock to prevent a container being removed many times
	// only the first to remove can be done
	// lock timeout should equal stop timeout
	lock, err := c.Lock(ctx, fmt.Sprintf("rmcontainer_%s", container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return err
	}
	defer lock.Unlock(ctx)

	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 docker 自身的处理
	// 另外我怀疑 docker 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
	removeCtx, cancel := context.WithTimeout(ctx, c.config.GlobalTimeout)
	defer cancel()
	if err = container.Engine.ContainerStop(removeCtx, container.ID, nil); err != nil {
		return err
	}
	log.Debugf("[removeOneContainer] Container stopped %s", container.ID)

	rmOpts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err = container.Engine.ContainerRemove(ctx, container.ID, rmOpts)
	if err != nil {
		return err
	}
	log.Debugf("[removeOneContainer] Container removed %s", container.ID)

	return c.store.RemoveContainer(ctx, container)
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
