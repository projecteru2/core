package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// GetWorkload get a workload
func (c *Calcium) GetWorkload(ctx context.Context, ID string) (workload *types.Workload, err error) {
	logger := log.WithFunc("calcium.GetWorkload").WithField("ID", ID)
	if ID == "" {
		logger.Error(ctx, types.ErrEmptyWorkloadID)
		return workload, types.ErrEmptyWorkloadID
	}
	workload, err = c.store.GetWorkload(ctx, ID)
	logger.Error(ctx, err)
	return workload, err
}

// GetWorkloads get workloads
func (c *Calcium) GetWorkloads(ctx context.Context, IDs []string) (workloads []*types.Workload, err error) {
	workloads, err = c.store.GetWorkloads(ctx, IDs)
	log.WithFunc("calcium.GetWorkloads").WithField("IDs", IDs).Error(ctx, err)
	return workloads, err
}

// ListWorkloads list workloads
func (c *Calcium) ListWorkloads(ctx context.Context, opts *types.ListWorkloadsOptions) (workloads []*types.Workload, err error) {
	if workloads, err = c.store.ListWorkloads(ctx, opts.Appname, opts.Entrypoint, opts.Nodename, opts.Limit, opts.Labels); err != nil {
		log.WithFunc("calcium.ListWorkloads").WithField("opts", opts).Errorf(ctx, err, "Calcium.ListWorkloads] store list failed: %+v", err)
	}
	return workloads, err
}

// ListNodeWorkloads list workloads belong to one node
func (c *Calcium) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) (workloads []*types.Workload, err error) {
	logger := log.WithFunc("calcium.ListNodeWorkloads").WithField("node", nodename).WithField("labels", labels)
	if nodename == "" {
		logger.Error(ctx, types.ErrEmptyNodeName)
		return workloads, types.ErrEmptyNodeName
	}
	workloads, err = c.store.ListNodeWorkloads(ctx, nodename, labels)
	logger.Error(ctx, err)
	return workloads, err
}
