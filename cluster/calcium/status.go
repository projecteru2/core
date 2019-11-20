package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// GetContainersStatus get containers status
func (c *Calcium) GetContainersStatus(ctx context.Context, IDs []string) (map[string][]byte, error) {
	result := map[string][]byte{}
	for _, ID := range IDs {
		status, err := c.store.GetContainerStatus(ctx, ID)
		if err != nil {
			return nil, err
		}
		result[ID] = status
	}
	return result, nil
}

// SetContainersStatus set containers status
func (c *Calcium) SetContainersStatus(ctx context.Context, status map[string][]byte, ttls map[string]int64) (map[string][]byte, error) {
	result := map[string][]byte{}
	for ID, containerStatus := range status {
		container, err := c.store.GetContainer(ctx, ID)
		if err != nil {
			return nil, err
		}
		ttl, ok := ttls[ID]
		if !ok {
			ttl = 0
		}
		if err = c.store.SetContainerStatus(ctx, container, containerStatus, ttl); err != nil {
			return nil, err
		}
		result[ID] = containerStatus
	}
	return result, nil
}

// DeployStatusStream watch deploy status
func (c *Calcium) DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus {
	return c.store.WatchDeployStatus(ctx, appname, entrypoint, nodename)
}
