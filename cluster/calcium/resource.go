package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
)

// PodResource show pod resource usage
func (c *Calcium) PodResource(ctx context.Context, podname string) (chan *types.NodeResource, error) {
	logger := log.WithField("Calcium", "PodResource").WithField("podname", podname)
	nodeCh, err := c.ListPodNodes(ctx, &types.ListNodesOptions{Podname: podname, All: true})
	if err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}
	ch := make(chan *types.NodeResource)

	go func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		for node := range nodeCh {
			node := node
			wg.Add(1)
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				nodeResource, err := c.doGetNodeResource(ctx, node.Name, false)
				if err != nil {
					nodeResource = &types.NodeResource{
						Name: node.Name, Diffs: []string{logger.ErrWithTracing(ctx, err).Error()},
					}
				}
				ch <- nodeResource
			})
		}
		wg.Wait()
	}()
	return ch, nil
}

// NodeResource check node's workload and resource
func (c *Calcium) NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	logger := log.WithField("Calcium", "NodeResource").WithField("nodename", nodename).WithField("fix", fix)
	if nodename == "" {
		return nil, logger.ErrWithTracing(ctx, types.ErrEmptyNodeName)
	}

	nr, err := c.doGetNodeResource(ctx, nodename, fix)
	if err != nil {
		return nil, logger.ErrWithTracing(ctx, err)
	}
	for _, workload := range nr.Workloads {
		if _, err := workload.Inspect(ctx); err != nil { // 用于探测节点上容器是否存在
			nr.Diffs = append(nr.Diffs, fmt.Sprintf("workload %s inspect failed %v \n", workload.ID, err))
			continue
		}
	}
	return nr, logger.ErrWithTracing(ctx, err)
}

func (c *Calcium) doGetNodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	var nr *types.NodeResource
	return nr, c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
		workloads, err := c.ListNodeWorkloads(ctx, nodename, nil)
		if err != nil {
			log.Errorf(ctx, "[doGetNodeResource] failed to list node workloads, node %v, err: %v", nodename, err)
			return err
		}

		if fix {
			go c.SendNodeMetrics(ctx, node.Name)
		}

		nr = &types.NodeResource{
			Name:             nodename,
			ResourceCapacity: node.ResourceCapacity,
			ResourceUsage:    node.ResourceUsage,
			Diffs:            node.Diffs,
			Workloads:        workloads,
		}
		return nil
	})
}

func (c *Calcium) doGetDeployMap(ctx context.Context, nodes []string, opts *types.DeployOptions) (map[string]int, error) {
	// get nodes with capacity > 0
	nodeResourceInfoMap, total, err := c.rmgr.GetNodesDeployCapacity(ctx, nodes, opts.ResourceOpts)
	if err != nil {
		log.Errorf(ctx, "[doGetDeployMap] failed to select available nodes, nodes %v, err %v", nodes, err)
		return nil, errors.WithStack(err)
	}

	// get deployed & processing workload count on each node
	deployStatusMap, err := c.store.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	if err != nil {
		log.Errorf(ctx, "failed to get deploy status for %v_%v, err %v", opts.Name, opts.Entrypoint.Name, err)
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}

	return deployMap, nil
}
