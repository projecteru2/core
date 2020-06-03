package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"

	"github.com/projecteru2/core/types"
)

// ListContainers list containers
func (c *Calcium) ListContainers(ctx context.Context, opts *types.ListContainersOptions) ([]*types.Container, error) {
	return c.store.ListContainers(ctx, opts.Appname, opts.Entrypoint, opts.Nodename, opts.Limit, opts.Labels)
}

// ListNodeContainers list containers belong to one node
func (c *Calcium) ListNodeContainers(ctx context.Context, nodename string, labels map[string]string) ([]*types.Container, error) {
	return c.store.ListNodeContainers(ctx, nodename, labels)
}

func (c *Calcium) getContainerNode(ctx context.Context, ID string) (*types.Node, error) {
	con, err := c.GetContainer(ctx, ID)
	if err != nil {
		return nil, err
	}
	return c.GetNode(ctx, con.Nodename)
}

// GetContainer get a container
func (c *Calcium) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	return c.store.GetContainer(ctx, ID)
}

// GetContainers get containers
func (c *Calcium) GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error) {
	return c.store.GetContainers(ctx, IDs)
}
