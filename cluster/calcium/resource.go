package calcium

import (
	"context"
	"fmt"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) PodResource(ctx context.Context, podname string) (chan *types.NodeResourceInfo, error) {
	logger := log.WithFunc("calcium.PodResource").WithField("podname", podname)
	nodes, err := c.store.GetNodesByPod(ctx, &types.NodeFilter{Podname: podname}, false)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	return perNode(c, nodes, func(node *types.Node, ch chan<- *types.NodeResourceInfo) {
		nr, err := c.doGetNodeResource(ctx, node.Name, false, false)
		if err != nil {
			logger.Error(ctx, err)
			nr = &types.NodeResourceInfo{
				Name: node.Name, Diffs: []string{err.Error()},
			}
		}
		ch <- nr
	}), nil
}

func (c *Calcium) NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResourceInfo, error) {
	logger := log.WithFunc("calcium.NodeResource").WithField("node", nodename).WithField("fix", fix)
	nr, err := c.doGetNodeResource(ctx, nodename, true, fix)
	logger.Error(ctx, err)
	return nr, err
}

func (c *Calcium) doGetNodeResource(ctx context.Context, nodename string, inspect, fix bool) (*types.NodeResourceInfo, error) {
	logger := log.WithFunc("calcium.doGetNodeResource").WithField("node", nodename).WithField("inspect", inspect).WithField("fix", fix)
	if nodename == "" {
		logger.Error(ctx, types.ErrEmptyNodeName)
		return nil, types.ErrEmptyNodeName
	}
	var nr *types.NodeResourceInfo
	return nr, c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
		workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
		if err != nil {
			logger.Errorf(ctx, err, "failed to list node workloads, node %s", node.Name)
			return err
		}

		resourceCapacity, resourceUsage, resourceDiffs, err := c.rmgr.GetNodeResourceInfo(ctx, node.Name, workloads, fix)
		if err != nil {
			logger.Errorf(ctx, err, "failed to get node resources, node %s", node.Name)
			return err
		}
		nr = &types.NodeResourceInfo{
			Name:      node.Name,
			Capacity:  resourceCapacity,
			Usage:     resourceUsage,
			Diffs:     resourceDiffs,
			Workloads: workloads,
		}

		if inspect {
			for _, workload := range nr.Workloads {
				if _, err := workload.Inspect(ctx); err != nil {
					nr.Diffs = append(nr.Diffs, fmt.Sprintf("workload %s inspect failed %v \n", workload.ID, err))
				}
			}
		}

		return nil
	})
}

func (c *Calcium) doGetDeployStrategy(ctx context.Context, nodenames []string, opts *types.DeployOptions) (map[string]int, error) {
	logger := log.WithFunc("calcium.doGetDeployStrategy").WithField("nodes", nodenames)
	nodeResourceInfoMap, total, err := c.rmgr.GetNodesDeployCapacity(ctx, nodenames, opts.Resources)
	if err != nil {
		logger.Errorf(ctx, err, "failed to select available nodes, nodes %+v", nodenames)
		return nil, err
	}

	deployStatusMap, err := c.store.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	if err != nil {
		logger.Errorf(ctx, err, "failed to get deploy status for %s_%s", opts.Name, opts.Entrypoint.Name)
		return nil, err
	}

	strategyInfos := []strategy.Info{}
	for node, resourceInfo := range nodeResourceInfoMap {
		strategyInfos = append(strategyInfos, strategy.Info{
			Nodename: node,
			Usage:    resourceInfo.Usage,
			Rate:     resourceInfo.Rate,
			Capacity: resourceInfo.Capacity,
			Count:    deployStatusMap[node],
		})
	}

	deployMap, err := strategy.Deploy(ctx, opts.DeployStrategy, opts.Count, opts.NodesLimit, strategyInfos, total)
	if err != nil {
		return nil, err
	}

	return deployMap, nil
}
