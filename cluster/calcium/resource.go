package calcium

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// PodResource show pod resource usage
func (c *Calcium) PodResource(ctx context.Context, podname string) (chan *types.NodeResource, error) {
	logger := log.WithField("Calcium", "PodResource").WithField("podname", podname)
	nodeCh, err := c.ListPodNodes(ctx, &types.ListNodesOptions{Podname: podname, All: true})
	if err != nil {
		return nil, logger.Err(ctx, err)
	}
	ch := make(chan *types.NodeResource)
	pool := utils.NewGoroutinePool(int(c.config.MaxConcurrency))
	go func() {
		defer close(ch)
		for node := range nodeCh {
			nodename := node.Name
			pool.Go(ctx, func() {
				nodeResource, err := c.doGetNodeResource(ctx, nodename, false)
				if err != nil {
					nodeResource = &types.NodeResource{
						Name: nodename, Diffs: []string{logger.Err(ctx, err).Error()},
					}
				}
				ch <- nodeResource
			})
		}
		pool.Wait(ctx)
	}()
	return ch, nil
}

// NodeResource check node's workload and resource
func (c *Calcium) NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	logger := log.WithField("Calcium", "NodeResource").WithField("nodename", nodename).WithField("fix", fix)
	if nodename == "" {
		return nil, logger.Err(ctx, types.ErrEmptyNodeName)
	}

	nr, err := c.doGetNodeResource(ctx, nodename, fix)
	if err != nil {
		return nil, logger.Err(ctx, err)
	}
	for _, workload := range nr.Workloads {
		if _, err := workload.Inspect(ctx); err != nil { // 用于探测节点上容器是否存在
			nr.Diffs = append(nr.Diffs, fmt.Sprintf("workload %s inspect failed %v \n", workload.ID, err))
			continue
		}
	}
	return nr, logger.Err(ctx, err)
}

func (c *Calcium) doGetNodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	var nr *types.NodeResource
	return nr, c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
		workloads, err := c.ListNodeWorkloads(ctx, nodename, nil)
		if err != nil {
			log.Errorf(ctx, "[doGetNodeResource] failed to list node workloads, node %v, err: %v", nodename, err)
			return err
		}

		// TODO: percentage?
		resourceCapacity, resourceUsage, diffs, err := c.resource.GetNodeResourceInfo(ctx, nodename, workloads, fix)
		if err != nil {
			log.Errorf(ctx, "[doGetNodeResource] failed to get node resource, node %v, err: %v", nodename, err)
			return err
		}

		if fix {
			go c.SendNodeMetrics(ctx, node.Name)
		}

		nr = &types.NodeResource{
			Name:             nodename,
			ResourceCapacity: resourceCapacity,
			ResourceUsage:    resourceUsage,
			Diffs:            diffs,
			Workloads:        workloads,
		}
		return nil
	})
}

func (c *Calcium) doGetDeployMap(ctx context.Context, nodes []string, opts *types.DeployOptions) (map[string]int, error) {
	// get nodes with capacity > 0
	nodeResourceInfoMap, total, err := c.resource.GetNodesDeployCapacity(ctx, nodes, opts.ResourceOpts)
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

type remapMsg struct {
	id  string
	err error
}

// called on changes of resource binding, such as cpu binding
// as an internal api, remap doesn't lock node, the responsibility of that should be taken on by caller
func (c *Calcium) remapResource(ctx context.Context, node *types.Node) (ch chan *remapMsg, err error) {
	workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
	if err != nil {
		return
	}

	workloadMap := map[string]*types.Workload{}
	for _, workload := range workloads {
		workloadMap[workload.ID] = workload
	}

	engineArgsMap, err := c.resource.GetRemapArgs(ctx, node.Name, workloadMap)
	if err != nil {
		return nil, err
	}

	ch = make(chan *remapMsg, len(engineArgsMap))
	go func() {
		defer close(ch)
		for workloadID, engineArgs := range engineArgsMap {
			ch <- &remapMsg{
				id:  workloadID,
				err: node.Engine.VirtualizationUpdateResource(ctx, workloadID, &enginetypes.VirtualizationResource{EngineArgs: engineArgs}),
			}
		}
	}()

	return ch, nil
}

func (c *Calcium) doRemapResourceAndLog(ctx context.Context, logger log.Fields, node *types.Node) {
	log.Debugf(ctx, "[doRemapResourceAndLog] remap node %s", node.Name)
	ctx, cancel := context.WithTimeout(utils.InheritTracingInfo(ctx, context.TODO()), c.config.GlobalTimeout)
	defer cancel()

	err := c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
		logger = logger.WithField("Calcium", "doRemapResourceAndLog").WithField("nodename", node.Name)
		if ch, err := c.remapResource(ctx, node); logger.Err(ctx, err) == nil {
			for msg := range ch {
				log.Infof(ctx, "[doRemapResourceAndLog] id %v", msg.id)
				logger.WithField("id", msg.id).Err(ctx, msg.err) // nolint:errcheck
			}
		}
		return nil
	})

	if err != nil {
		log.Errorf(ctx, "[doRemapResourceAndLog] remap node %s failed, err: %v", node.Name, err)
	}
}
