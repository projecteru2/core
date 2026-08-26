package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	return c.store.GetNodeStatus(ctx, nodename)
}

func (c *Calcium) SetNodeStatus(ctx context.Context, nodename string, ttl int64) error {
	logger := log.WithFunc("calcium.SetNodeStatus").WithField("node", nodename).WithField("ttl", ttl)
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err)
		return err
	}
	err = c.store.SetNodeStatus(ctx, node, ttl)
	logger.Error(ctx, err)
	return err
}

func (c *Calcium) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	return c.store.NodeStatusStream(ctx)
}

func (c *Calcium) GetWorkloadsStatus(ctx context.Context, IDs []string) ([]*types.StatusMeta, error) {
	workloads, err := c.store.GetWorkloads(ctx, IDs)
	if err != nil {
		log.WithFunc("calcium.GetWorkloadsStatus").WithField("IDs", IDs).Error(ctx, err)
		return nil, err
	}
	return utils.Map(workloads, func(workload *types.Workload) *types.StatusMeta { return workload.StatusMeta }), nil
}

func (c *Calcium) SetWorkloadsStatus(ctx context.Context, statusMetas []*types.StatusMeta, ttls map[string]int64) ([]*types.StatusMeta, error) {
	logger := log.WithFunc("calcium.SetWorkloadsStatus").WithField("count", len(statusMetas)).WithField("ttls", ttls)
	// old callers omit appname, nodename and entrypoint; look them up
	missing := []string{}
	for _, statusMeta := range statusMetas {
		if statusMeta.Appname == "" || statusMeta.Nodename == "" || statusMeta.Entrypoint == "" {
			missing = append(missing, statusMeta.ID)
		}
	}
	workloads := map[string]*types.Workload{}
	if len(missing) > 0 {
		ws, err := c.store.GetWorkloads(ctx, missing)
		if err != nil {
			logger.Error(ctx, err)
			return nil, err
		}
		for _, workload := range ws {
			workloads[workload.ID] = workload
		}
	}

	r := []*types.StatusMeta{}
	for _, statusMeta := range statusMetas {
		if workload, ok := workloads[statusMeta.ID]; ok {
			appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
			if err != nil {
				logger.Error(ctx, err)
				return nil, err
			}
			statusMeta.Appname = appname
			statusMeta.Nodename = workload.Nodename
			statusMeta.Entrypoint = entrypoint
		}

		ttl := ttls[statusMeta.ID]
		if err := c.store.SetWorkloadStatus(ctx, statusMeta, ttl); err != nil {
			logger.Error(ctx, err)
			return nil, err
		}
		r = append(r, statusMeta)
	}
	return r, nil
}

func (c *Calcium) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	return c.store.WorkloadStatusStream(ctx, appname, entrypoint, nodename, labels)
}
