package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// GetWorkloadsStatus get workload status
func (c *Calcium) GetWorkloadsStatus(ctx context.Context, ids []string) ([]*types.StatusMeta, error) {
	r := []*types.StatusMeta{}
	for _, id := range ids {
		s, err := c.store.GetWorkloadStatus(ctx, id)
		if err != nil {
			return r, err
		}
		r = append(r, s)
	}
	return r, nil
}

// SetWorkloadsStatus set workloads status
func (c *Calcium) SetWorkloadsStatus(ctx context.Context, status []*types.StatusMeta, ttls map[string]int64) ([]*types.StatusMeta, error) {
	r := []*types.StatusMeta{}
	for _, workloadStatus := range status {
		workload, err := c.store.GetWorkload(ctx, workloadStatus.ID)
		if err != nil {
			return nil, err
		}
		ttl, ok := ttls[workloadStatus.ID]
		if !ok {
			ttl = 0
		}
		workload.StatusMeta = workloadStatus
		if err = c.store.SetWorkloadStatus(ctx, workload, ttl); err != nil {
			return nil, err
		}
		r = append(r, workload.StatusMeta)
	}
	return r, nil
}

// WorkloadStatusStream stream workload status
func (c *Calcium) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	return c.store.WorkloadStatusStream(ctx, appname, entrypoint, nodename, labels)
}

// SetNodeStatus set status of a node
// it's used to report whether a node is still alive
func (c *Calcium) SetNodeStatus(ctx context.Context, nodename string, ttl int64) error {
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		return err
	}
	return c.store.SetNodeStatus(ctx, node, ttl)
}

// NodeStatusStream returns a stream of node status for subscribing
func (c *Calcium) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	return c.store.NodeStatusStream(ctx)
}
