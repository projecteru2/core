package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

// PodResource show pod resource usage
func (c *Calcium) PodResource(ctx context.Context, podname string) (chan *types.NodeResourceInfo, error) {
	logger := log.WithFunc("calcium.PodResource").WithField("podname", podname)
	nodes, err := c.store.GetNodesByPod(ctx, &types.NodeFilter{Podname: podname})
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	ch := make(chan *types.NodeResourceInfo)

	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(nodes))
		defer wg.Wait()
		for _, node := range nodes {
			node := node
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				nr, err := c.doGetNodeResource(ctx, node.Name, false, false)
				if err != nil {
					logger.Error(ctx, err)
					nr = &types.NodeResourceInfo{
						Name: node.Name, Diffs: []string{err.Error()},
					}
				}
				ch <- nr
			})
		}
	})

	return ch, nil
}

// NodeResource check node's workload and resource
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
			logger.Errorf(ctx, err, "failed to list node workloads, node %+v", node.Name)
			return err
		}

		// get node resources
		resourceCapacity, resourceUsage, resourceDiffs, err := c.rmgr.GetNodeResourceInfo(ctx, node.Name, workloads, fix)
		if err != nil {
			logger.Errorf(ctx, err, "failed to get node resources, node %+v", node.Name)
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
				if _, err := workload.Inspect(ctx); err != nil { // 用于探测节点上容器是否存在
					nr.Diffs = append(nr.Diffs, fmt.Sprintf("workload %s inspect failed %+v \n", workload.ID, err))
					continue
				}
			}
		}

		return nil
	})
}

func (c *Calcium) doGetDeployStrategy(ctx context.Context, nodenames []string, opts *types.DeployOptions) (map[string]int, error) {
	logger := log.WithFunc("calcium.doGetDeployStrategy").WithField("nodes", nodenames)
	// get nodes with capacity > 0
	nodeResourceInfoMap, total, err := c.rmgr.GetNodesDeployCapacity(ctx, nodenames, opts.Resources)
	if err != nil {
		logger.Errorf(ctx, err, "failed to select available nodes, nodes %+v", nodenames)
		return nil, err
	}

	// get deployed & processing workload count on each node
	deployStatusMap, err := c.store.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	if err != nil {
		logger.Errorf(ctx, err, "failed to get deploy status for %+v_%+v", opts.Name, opts.Entrypoint.Name)
		return nil, err
	}

	// generate strategy info
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

	// generate deploy plan
	deployMap, err := strategy.Deploy(ctx, opts.DeployStrategy, opts.Count, opts.NodesLimit, strategyInfos, total)
	if err != nil {
		return nil, err
	}

	return deployMap, nil
}
