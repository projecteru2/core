package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// GetNodeStatus set status of a node
// it's used to report whether a node is still alive
func (c *Calcium) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	return c.store.GetNodeStatus(ctx, nodename)
}

// SetNodeStatus set status of a node
// it's used to report whether a node is still alive
func (c *Calcium) SetNodeStatus(ctx context.Context, nodename string, ttl int64) error {
	logger := log.WithField("Calcium", "SetNodeStatus").WithField("nodename", nodename).WithField("ttl", ttl)
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		return logger.ErrWithTracing(ctx, errors.WithStack(err))
	}
	return logger.ErrWithTracing(ctx, errors.WithStack(c.store.SetNodeStatus(ctx, node, ttl)))
}

// NodeStatusStream returns a stream of node status for subscribing
func (c *Calcium) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	return c.store.NodeStatusStream(ctx)
}

// GetWorkloadsStatus get workload status
func (c *Calcium) GetWorkloadsStatus(ctx context.Context, ids []string) ([]*types.StatusMeta, error) {
	r := []*types.StatusMeta{}
	for _, id := range ids {
		s, err := c.store.GetWorkloadStatus(ctx, id)
		if err != nil {
			return r, log.WithField("Calcium", "GetWorkloadStatus").WithField("ids", ids).ErrWithTracing(ctx, errors.WithStack(err))
		}
		r = append(r, s)
	}
	return r, nil
}

// SetWorkloadsStatus set workloads status
func (c *Calcium) SetWorkloadsStatus(ctx context.Context, statusMetas []*types.StatusMeta, ttls map[string]int64) ([]*types.StatusMeta, error) {
	logger := log.WithField("Calcium", "SetWorkloadsStatus").WithField("status", statusMetas[0]).WithField("ttls", ttls)
	r := []*types.StatusMeta{}
	for _, statusMeta := range statusMetas {
		// In order to compat
		if statusMeta.Appname == "" || statusMeta.Nodename == "" || statusMeta.Entrypoint == "" {
			workload, err := c.store.GetWorkload(ctx, statusMeta.ID)
			if err != nil {
				return nil, logger.ErrWithTracing(ctx, errors.WithStack(err))
			}

			appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
			if err != nil {
				return nil, logger.ErrWithTracing(ctx, errors.WithStack(err))
			}

			statusMeta.Appname = appname
			statusMeta.Nodename = workload.Nodename
			statusMeta.Entrypoint = entrypoint
		}

		ttl, ok := ttls[statusMeta.ID]
		if !ok {
			ttl = 0
		}

		if err := c.store.SetWorkloadStatus(ctx, statusMeta, ttl); err != nil {
			return nil, logger.ErrWithTracing(ctx, errors.WithStack(err))
		}
		r = append(r, statusMeta)
	}
	return r, nil
}

// WorkloadStatusStream stream workload status
func (c *Calcium) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	return c.store.WorkloadStatusStream(ctx, appname, entrypoint, nodename, labels)
}
