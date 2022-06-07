package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"

	"github.com/pkg/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ListWorkloads list workloads
func (c *Calcium) ListWorkloads(ctx context.Context, opts *types.ListWorkloadsOptions) (workloads []*types.Workload, err error) {
	if workloads, err = c.store.ListWorkloads(ctx, opts.Appname, opts.Entrypoint, opts.Nodename, opts.Limit, opts.Labels); err != nil {
		log.WithField("opts", opts).Errorf(ctx, "Calcium.ListWorkloads] store list failed: %+v", err)
	}
	return workloads, errors.WithStack(err)
}

// ListNodeWorkloads list workloads belong to one node
func (c *Calcium) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) (workloads []*types.Workload, err error) {
	logger := log.WithField("Calcium", "ListNodeWorkloads").WithField("nodename", nodename).WithField("labels", labels)
	if nodename == "" {
		return workloads, logger.Err(ctx, errors.WithStack(types.ErrEmptyNodeName))
	}
	workloads, err = c.store.ListNodeWorkloads(ctx, nodename, labels)
	return workloads, logger.Err(ctx, errors.WithStack(err))
}

func (c *Calcium) getWorkloadNode(ctx context.Context, id string) (*types.Node, error) {
	w, err := c.GetWorkload(ctx, id)
	if err != nil {
		return nil, err
	}
	node, err := c.GetNode(ctx, w.Nodename)
	return node, err
}

// GetWorkload get a workload
func (c *Calcium) GetWorkload(ctx context.Context, id string) (workload *types.Workload, err error) {
	logger := log.WithField("Calcium", "GetWorkload").WithField("id", id)
	if id == "" {
		return workload, logger.Err(ctx, errors.WithStack(types.ErrEmptyWorkloadID))
	}
	workload, err = c.store.GetWorkload(ctx, id)
	return workload, logger.Err(ctx, errors.WithStack(err))
}

// GetWorkloads get workloads
func (c *Calcium) GetWorkloads(ctx context.Context, ids []string) (workloads []*types.Workload, err error) {
	workloads, err = c.store.GetWorkloads(ctx, ids)
	return workloads, log.WithField("Calcium", "GetWorkloads").WithField("ids", ids).Err(ctx, errors.WithStack(err))
}
