package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// GetNodeStatus set status of a node
// it's used to report whether a node is still alive
func (c *Calcium) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	return c.store.GetNodeStatus(ctx, nodename)
}

// SetNodeStatus set status of a node
// it's used to report whether a node is still alive
func (c *Calcium) SetNodeStatus(ctx context.Context, nodename string, ttl int64) error {
	logger := log.WithFunc("calcium.SetNodeStatus").WithField("nodename", nodename).WithField("ttl", ttl)
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	err = c.store.SetNodeStatus(ctx, node, ttl)
	logger.Error(ctx, err)
	return err
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
			log.WithFunc("calcium.GetWorkloadStatus").WithField("ids", ids).Error(ctx, err)
			return r, err
		}
		r = append(r, s)
	}
	return r, nil
}

// SetWorkloadsStatus set workloads status
func (c *Calcium) SetWorkloadsStatus(ctx context.Context, statusMetas []*types.StatusMeta, ttls map[string]int64) ([]*types.StatusMeta, error) {
	logger := log.WithFunc("calcium.SetWorkloadsStatus").WithField("status", statusMetas[0]).WithField("ttls", ttls)
	r := []*types.StatusMeta{}
	for _, statusMeta := range statusMetas {
		// In order to compat
		if statusMeta.Appname == "" || statusMeta.Nodename == "" || statusMeta.Entrypoint == "" {
			workload, err := c.store.GetWorkload(ctx, statusMeta.ID)
			if err != nil {
				logger.Error(ctx, err)
				return nil, err
			}

			appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
			if err != nil {
				logger.Error(ctx, err)
				return nil, err
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
			logger.Error(ctx, err)
			return nil, err
		}
		r = append(r, statusMeta)
	}
	return r, nil
}

// WorkloadStatusStream stream workload status
func (c *Calcium) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	return c.store.WorkloadStatusStream(ctx, appname, entrypoint, nodename, labels)
}
