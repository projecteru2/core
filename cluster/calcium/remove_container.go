package calcium

import (
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

	containers, err := c.GetContainers(ids)
	if err != nil {
		return ch, err
	}

	go func() {
		wg := sync.WaitGroup{}
		wg.Add(len(containers))

		for _, container := range containers {
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
	log.WithFields(log.Fields{"nodename": node.Name, "cpumap": container.CPU}).Debugln("Restore node CPU:")
	if container.CPU.Total() > 0 {
		if err := c.store.UpdateNodeCPU(node.Podname, node.Name, container.CPU, "+"); err != nil {
			return err
		}
	}
	return c.store.RemoveContainer(info.ID)
}
