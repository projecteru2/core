package cluster

import (
	"sync"
	"time"

	enginetypes "github.com/docker/engine-api/types"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

// remove containers
// returns a channel that contains removing responses
func (c *Calcium) RemoveContainer(ids []string) (chan *types.RemoveContainerMessage, error) {
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

				if err := removeOneContainer(container); err != nil {
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
func removeOneContainer(c *types.Container) error {
	info, err := c.Inspect()
	if err != nil {
		return err
	}

	err = c.Engine.ContainerStop(context.Background(), c.ID, 5*time.Second)
	if err != nil {
		return err
	}

	opts := enginetypes.ContainerRemoveOptions{
		RemoveVolumes: true,
		RemoveLinks:   true,
		Force:         true,
	}
	err = c.Engine.ContainerRemove(context.Background(), c.ID, opts)
	if err != nil {
		return err
	}

	// try to remove corresponding image
	// we don't care if some container is still using the image
	// docker will care that for us
	// TODO what if we use another worker to clean all the images?
	opts := enginetypes.ImageRemoveOptions{
		Force:         false,
		PruneChildren: true,
	}
	go c.Engine.ImageRemove(context.Background(), info.Image, opts)

	return c.store.RemoveContainer(id)
}
