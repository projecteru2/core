package calcium

import (
	"context"
	"fmt"
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
func (c *calcium) RemoveContainer(ids []string) (chan *types.RemoveContainerMessage, error) {
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
			if err != nil {
				ch <- &types.RemoveContainerMessage{
					ContainerID: id,
					Success:     false,
					Message:     err.Error(),
				}
				continue
			}

			wg.Add(1)
			ib.Add(container.Podname, info.Config.Image)
			go func(container *types.Container, info enginetypes.ContainerJSON) {
				defer wg.Done()

				success := true
				message := "success"

				if err := c.removeOneContainer(container, info); err != nil {
					success = false
					message = err.Error()
				}
				ch <- &types.RemoveContainerMessage{
					ContainerID: container.ID,
					Success:     success,
					Message:     message,
				}
			}(container, info)
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
func (c *calcium) removeOneContainer(container *types.Container, info enginetypes.ContainerJSON) error {
	// use etcd lock to prevent a container being removed many times
	// only the first to remove can be done
	// lock timeout should equal stop timeout
	lock, err := c.Lock(fmt.Sprintf("rmcontainer_%s", container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		log.Errorf("[removeOneContainer] Error during lock.Lock: %s", err.Error())
		return err
	}
	defer lock.Unlock()

	// will be used later to update
	node, err := c.GetNode(container.Podname, container.Nodename)
	if err != nil {
		log.Errorf("[removeOneContainer] Error during GetNode: %s", err.Error())
		return err
	}

	defer func() {
		// if total cpu of container > 0, then we need to release these core resource
		// but if it's 0, just ignore to save 1 time write on etcd.
		if container.CPU.Total() > 0 {
			log.Debugf("[removeOneContainer] Restore node cpu: %v, %v", node, container.CPU)
			if err := c.store.UpdateNodeCPU(node.Podname, node.Name, container.CPU, "+"); err != nil {
				log.Errorf("[removeOneContainer] Update Node CPU failed %v", err)
			}
			return
		}
		if err := c.store.UpdateNodeMem(node.Podname, node.Name, container.Memory, "+"); err != nil {
			log.Errorf("[removeOneContainer] Update Node Memory failed %v", err)
		}
	}()

	if container.Hook != nil && len(container.Hook.BeforeStop) > 0 {
		for _, cmd := range container.Hook.BeforeStop {
			output, err := execuateInside(container.Engine, container.ID, cmd, info.Config.User, info.Config.Env, container.Privileged)
			if err != nil {
				if container.Hook.Force {
					return err
				}
				log.Errorf("[removeOneContainer] hook error %v", err)
			}
			if len(output) > 0 {
				log.Infof("[removeOneContainer] hook output %s\n%s", cmd, output)
			}
		}
	}

	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 docker 自身的处理
	// 另外我怀疑 docker 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
	ctx, cancel := context.WithTimeout(context.Background(), c.config.GlobalTimeout)
	defer cancel()
	if err = container.Engine.ContainerStop(ctx, info.ID, nil); err != nil {
		log.Errorf("[removeOneContainer] Error during ContainerStop: %s", err.Error())
		return err
	}
	log.Debugf("[removeOneContainer] Container stopped %s", info.ID)

	rmOpts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err = container.Engine.ContainerRemove(context.Background(), info.ID, rmOpts)
	if err != nil {
		log.Errorf("[removeOneContainer] Error during ContainerRemove: %s", err.Error())
		return err
	}
	log.Debugf("[removeOneContainer] Container removed %s", info.ID)

	return c.store.RemoveContainer(container)
}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *calcium) removeContainerSync(ids []string) error {
	ch, err := c.RemoveContainer(ids)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[removeContainerSync] Removed %s", m.ContainerID)
	}
	return nil
}
