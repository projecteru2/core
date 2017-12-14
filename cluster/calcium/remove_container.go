package calcium

import (
	"context"
	"fmt"
	"strings"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
)

type imageBucket struct {
	sync.Mutex
	data map[string]map[string]struct{}
}

func newImageBucket() *imageBucket {
	return &imageBucket{data: make(map[string]map[string]struct{})}
}

func (ib *imageBucket) Add(podname, image string) {
	ib.Lock()
	defer ib.Unlock()

	if _, ok := ib.data[podname]; !ok {
		ib.data[podname] = make(map[string]struct{})
	}
	ib.data[podname][image] = struct{}{}
}

func (ib *imageBucket) Dump() map[string][]string {
	r := make(map[string][]string)
	for podname, imageMap := range ib.data {
		images := []string{}
		for image := range imageMap {
			images = append(images, image)
		}
		r[podname] = images
	}
	return r
}

// remove containers
// returns a channel that contains removing responses
func (c *calcium) RemoveContainer(ids []string, force bool) (chan *types.RemoveContainerMessage, error) {
	ch := make(chan *types.RemoveContainerMessage)
	go func() {
		wg := sync.WaitGroup{}
		ib := newImageBucket()

		for _, id := range ids {
			// 单个单个取是因为某些情况可能会传了id但是这里没有
			// 这种情况不希望直接打断操作, 而是希望错误在message里回去.
			container, err := c.GetContainer(id)
			if err != nil {
				ch <- &types.RemoveContainerMessage{
					ContainerID: id,
					Success:     false,
					Message:     err.Error(),
				}
				continue
			}

			info, err := container.Inspect()
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
						output, err := execuateInside(container.Engine, container.ID, cmd, info.Config.User, info.Config.Env, container.Privileged)
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

				if err := c.removeOneContainer(container); err != nil {
					success = false
					message += err.Error()
				}

			}(container, info, message)
		}

		wg.Wait()

		// 把收集的image清理掉
		go func(ib *imageBucket) {
			for podname, images := range ib.Dump() {
				for _, image := range images {
					c.cleanImage(podname, image)
				}
			}
		}(ib)
		close(ch)
	}()

	return ch, nil

}

// remove one container
func (c *calcium) removeOneContainer(container *types.Container) error {
	defer func() {
		// if total cpu of container > 0, then we need to release these core resource
		// but if it's 0, just ignore to save 1 time write on etcd.
		if container.CPU.Total() > 0 {
			log.Debugf("[removeOneContainer] Restore node %s cpu: %v", container.Nodename, container.CPU)
			if err := c.store.UpdateNodeCPU(container.Podname, container.Nodename, container.CPU, "+"); err != nil {
				log.Errorf("[removeOneContainer] Update Node CPU failed %v", err)
			}
			return
		}
		if container.Memory > 0 {
			log.Debugf("[removeOneContainer] Restore node %s memory: %d", container.Nodename, container.Memory)
			if err := c.store.UpdateNodeMem(container.Podname, container.Nodename, container.Memory, "+"); err != nil {
				log.Errorf("[removeOneContainer] Update Node Memory failed %v", err)
			}
		}
	}()

	// 没 ID 就只做回收
	if container.ID == "" {
		return nil
	}

	// use etcd lock to prevent a container being removed many times
	// only the first to remove can be done
	// lock timeout should equal stop timeout
	lock, err := c.Lock(fmt.Sprintf("rmcontainer_%s", container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return err
	}
	defer lock.Unlock()

	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 docker 自身的处理
	// 另外我怀疑 docker 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
	ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalTimeout)
	defer cancel()
	if err = container.Engine.ContainerStop(ctx, container.ID, nil); err != nil {
		return err
	}
	log.Debugf("[removeOneContainer] Container stopped %s", container.ID)

	rmOpts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err = container.Engine.ContainerRemove(context.Background(), container.ID, rmOpts)
	if err != nil {
		return err
	}
	log.Debugf("[removeOneContainer] Container removed %s", container.ID)

	return c.store.RemoveContainer(container)
}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *calcium) removeContainerSync(ids []string) error {
	ch, err := c.RemoveContainer(ids, true)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[removeContainerSync] Removed %s", m.ContainerID)
	}
	return nil
}
