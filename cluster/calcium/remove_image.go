package calcium

import (
	"context"
	"fmt"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
)

// remove images
func (c *calcium) RemoveImage(ctx context.Context, podname, nodename string, images []string) (chan *types.RemoveImageMessage, error) {
	ch := make(chan *types.RemoveImageMessage)

	node, err := c.GetNode(podname, nodename)
	if err != nil {
		return ch, err
	}

	opts := enginetypes.ImageRemoveOptions{
		Force:         false,
		PruneChildren: true,
	}

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(images))
		defer wg.Wait()

		for _, image := range images {
			go func(image string) {
				defer wg.Done()

				messages := []string{}
				success := true
				ms, err := node.Engine.ImageRemove(ctx, image, opts)
				if err != nil {
					success = false
					messages = append(messages, err.Error())
				} else {
					for _, m := range ms {
						if m.Untagged != "" {
							messages = append(messages, fmt.Sprintf("Untagged: %s", m.Untagged))
						}
						if m.Deleted != "" {
							messages = append(messages, fmt.Sprintf("Deleted: %s", m.Deleted))
						}
					}
				}
				ch <- &types.RemoveImageMessage{
					Image:    image,
					Success:  success,
					Messages: messages,
				}
			}(image)
		}
	}()

	return ch, nil
}
