package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// SetContainersStatus set containers status
func (c *Calcium) SetContainersStatus(ctx context.Context, status map[string][]byte, ttls map[string]int64) error {
	for ID, containerStatus := range status {
		container, err := c.store.GetContainer(ctx, ID)
		if err != nil {
			return err
		}
		ttl, ok := ttls[ID]
		if !ok {
			ttl = 0
		}
		if err = c.store.SetContainerStatus(ctx, container, containerStatus, ttl); err != nil {
			return err
		}
	}
	return nil
}

// ContainerStatusStream stream container status
func (c *Calcium) ContainerStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.ContainerStatus {
	return c.store.ContainerStatusStream(ctx, appname, entrypoint, nodename, labels)
}
