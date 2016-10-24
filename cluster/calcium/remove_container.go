package calcium

import (
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/engine-api/types"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

// remove containers
// returns a channel that contains removing responses
func (c *calcium) RemoveContainer(ids []string) (chan *types.RemoveContainerMessage, error) {
	ch := make(chan *types.RemoveContainerMessage)
	go func() {
		wg := sync.WaitGroup{}

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

			wg.Add(1)
			go func(container *types.Container) {
				defer wg.Done()

				success := true
				message := "success"

				if err := c.removeOneContainer(container); err != nil {
					success = false
					message = err.Error()
				}
				ch <- &types.RemoveContainerMessage{
					ContainerID: container.ID,
					Success:     success,
					Message:     message,
				}
			}(container)
		}

		wg.Wait()
		close(ch)
	}()

	return ch, nil

}

// remove one container
// 5 seconds timeout
func (c *calcium) removeOneContainer(container *types.Container) error {
	// use etcd lock to prevent a container being removed many times
	// only the first to remove can be done
	lock, err := c.store.CreateLock(fmt.Sprintf("rmcontainer_%s", container.ID), 120)
	if err != nil {
		return err
	}
	if err := lock.Lock(); err != nil {
		return err
	}
	defer lock.Unlock()

	info, err := container.Inspect()
	if err != nil {
		return err
	}

	// will be used later to update
	node, err := c.GetNode(container.Podname, container.Nodename)
	if err != nil {
		return err
	}

	timeout := 5 * time.Second
	err = container.Engine.ContainerStop(context.Background(), info.ID, &timeout)
	if err != nil {
		return err
	}

	rmOpts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	}
	err = container.Engine.ContainerRemove(context.Background(), info.ID, rmOpts)
	if err != nil {
		return err
	}

	// try to remove corresponding image
	// we don't care if some container is still using the image
	// docker will care that for us
	// TODO what if we use another worker to clean all the images?
	rmiOpts := enginetypes.ImageRemoveOptions{
		Force:         false,
		PruneChildren: true,
	}
	go container.Engine.ImageRemove(context.Background(), info.Image, rmiOpts)

	// if total cpu of container > 0, then we need to release these core resource
	// but if it's 0, just ignore to save 1 time write on etcd.
	if container.CPU.Total() > 0 {
		log.WithFields(log.Fields{"nodename": node.Name, "cpumap": container.CPU}).Debugln("Restore node CPU:")
		if err := c.store.UpdateNodeCPU(node.Podname, node.Name, container.CPU, "+"); err != nil {
			return err
		}
	}
	c.store.UpdateNodeMem(node.Podname, node.Name, container.Memory, "+")
	return c.store.RemoveContainer(info.ID)
}
