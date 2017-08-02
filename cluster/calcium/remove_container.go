package calcium

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
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
// 5 seconds timeout
func (c *calcium) removeOneContainer(container *types.Container, info enginetypes.ContainerJSON) error {
	// use etcd lock to prevent a container being removed many times
	// only the first to remove can be done
	lock, err := c.Lock(fmt.Sprintf("rmcontainer_%s", container.ID), 120)
	if err != nil {
		log.Errorf("Error during lock.Lock: %s", err.Error())
		return err
	}
	defer lock.Unlock()

	// will be used later to update
	node, err := c.GetNode(container.Podname, container.Nodename)
	if err != nil {
		log.Errorf("Error during GetNode: %s", err.Error())
		return err
	}

	defer func() {
		// if total cpu of container > 0, then we need to release these core resource
		// but if it's 0, just ignore to save 1 time write on etcd.
		if container.CPU.Total() > 0 {
			log.WithFields(log.Fields{"nodename": node.Name, "cpumap": container.CPU}).Debugln("Restore node CPU:")
			if err := c.store.UpdateNodeCPU(node.Podname, node.Name, container.CPU, "+"); err != nil {
				log.Errorf("Update Node CPU failed %s", err.Error())
				return
			}
		}
		c.store.UpdateNodeMem(node.Podname, node.Name, container.Memory, "+")
	}()

	// before stop
	if err := runExec(container.Engine, info, BEFORE_STOP); err != nil {
		log.Errorf("Run exec at %s error: %s", BEFORE_STOP, err.Error())
	}

	timeout := 5 * time.Second
	err = container.Engine.ContainerStop(context.Background(), info.ID, &timeout)
	if err != nil {
		log.Errorf("Error during ContainerStop: %s", err.Error())
		return err
	}

	rmOpts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err = container.Engine.ContainerRemove(context.Background(), info.ID, rmOpts)
	if err != nil {
		log.Errorf("Error during ContainerRemove: %s", err.Error())
		return err
	}

	if err = c.store.RemoveContainer(info.ID, container); err != nil {
		log.Errorf("Error during remove etcd data: %s", err.Error())
		return err
	}
	return nil
}

// 同步地删除容器, 在某些需要等待的场合异常有用!
func (c *calcium) removeContainerSync(ids []string) error {
	ch, err := c.RemoveContainer(ids)
	if err != nil {
		return err
	}

	for m := range ch {
		log.Debugf("[removeContainerSync] %s", m.ContainerID)
	}
	return nil
}
