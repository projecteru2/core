package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// GetContainersStatus get container status
func (c *Calcium) GetContainersStatus(ctx context.Context, IDs []string) ([]types.StatusMeta, error) {
	r := []types.StatusMeta{}
	for _, ID := range IDs {
		s, err := c.store.GetContainerStatus(ctx, ID)
		if err != nil {
			return r, err
		}
		r = append(r, s)
	}
	return r, nil
}

// SetContainersStatus set containers status
func (c *Calcium) SetContainersStatus(ctx context.Context, status map[string]types.StatusMeta, ttls map[string]int64, force bool) ([]types.StatusMeta, error) {
	r := []types.StatusMeta{}
	for ID, containerStatus := range status {
		container, err := c.store.GetContainer(ctx, ID)
		if err != nil {
			return nil, err
		}
		ttl, ok := ttls[ID]
		if !ok {
			ttl = 0
		}
		container.StatusMeta = containerStatus
		if err = c.store.SetContainerStatus(ctx, container, ttl, force); err != nil {
			return nil, err
		}
		r = append(r, container.StatusMeta)
	}
	return r, nil
}

// ContainerStatusStream stream container status
func (c *Calcium) ContainerStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.ContainerStatus {
	return c.store.ContainerStatusStream(ctx, appname, entrypoint, nodename, labels)
}
