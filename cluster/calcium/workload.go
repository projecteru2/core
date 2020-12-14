package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"

	"github.com/projecteru2/core/types"
)

// ListWorkloads list workloads
func (c *Calcium) ListWorkloads(ctx context.Context, opts *types.ListWorkloadsOptions) ([]*types.Workload, error) {
	return c.store.ListWorkloads(ctx, opts.Appname, opts.Entrypoint, opts.Nodename, opts.Limit, opts.Labels)
}

// ListNodeWorkloads list workloads belong to one node
func (c *Calcium) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error) {
	if nodename == "" {
		return nil, types.ErrEmptyNodeName
	}
	return c.store.ListNodeWorkloads(ctx, nodename, labels)
}

func (c *Calcium) getWorkloadNode(ctx context.Context, id string) (*types.Node, error) {
	w, err := c.GetWorkload(ctx, id)
	if err != nil {
		return nil, err
	}
	return c.GetNode(ctx, w.Nodename)
}

// GetWorkload get a workload
func (c *Calcium) GetWorkload(ctx context.Context, id string) (*types.Workload, error) {
	if id == "" {
		return nil, types.ErrEmptyWorkloadID
	}
	return c.store.GetWorkload(ctx, id)
}

// GetWorkloads get workloads
func (c *Calcium) GetWorkloads(ctx context.Context, ids []string) ([]*types.Workload, error) {
	return c.store.GetWorkloads(ctx, ids)
}
