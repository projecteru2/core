package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) PodResource(ctx context.Context, podname string) (chan *types.NodeResourceInfo, error) {
	logger := log.WithFunc("calcium.PodResource").WithField("podname", podname)
	var ch chan *types.NodeResourceInfo
	err := c.withNodes(ctx, &types.NodeFilter{Podname: podname}, func(ctx context.Context, nodes map[string]*types.Node) error {
		ch = make(chan *types.NodeResourceInfo, len(nodes))
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(nodes))
		for _, node := range nodes {
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				nr, err := c.doComputeNodeResource(ctx, node.Name, false)
				if err != nil {
					logger.Error(ctx, err)
					nr = &types.NodeResourceInfo{
						Name: node.Name, Diffs: []string{err.Error()},
					}
				}
				ch <- nr
			})
		}
		wg.Wait()
		return nil
	})
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	return ch, nil
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
	compute := func(ctx context.Context, node *types.Node) (err error) {
		nr, err = c.doComputeNodeResource(ctx, node.Name, fix)
		return err
	}
	withNode := c.withNode
	if fix {
		withNode = c.withNodeOperationLocked
	}
	if err := withNode(ctx, nodename, compute); err != nil {
		return nil, err
	}

	if inspect {
		for _, workload := range nr.Workloads {
			if _, err := workload.Inspect(ctx); err != nil {
				nr.Diffs = append(nr.Diffs, fmt.Sprintf("workload %s inspect failed %v \n", workload.ID, err))
			}
		}
	}
	return nr, nil
}

func (c *Calcium) doComputeNodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResourceInfo, error) {
	logger := log.WithFunc("calcium.doComputeNodeResource").WithField("node", nodename).WithField("fix", fix)
	workloads, err := c.store.ListNodeWorkloads(ctx, nodename, nil)
	if err != nil {
		logger.Errorf(ctx, err, "failed to list node workloads, node %s", nodename)
		return nil, err
	}

	resourceCapacity, resourceUsage, resourceDiffs, err := c.rmgr.GetNodeResourceInfo(ctx, nodename, workloads, fix)
	if err != nil {
		logger.Errorf(ctx, err, "failed to get node resources, node %s", nodename)
		return nil, err
	}
	return &types.NodeResourceInfo{
		Name:      nodename,
		Capacity:  resourceCapacity,
		Usage:     resourceUsage,
		Diffs:     resourceDiffs,
		Workloads: workloads,
	}, nil
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
