package calcium

import (
	"context"
	"sync"

	enginefactory "github.com/projecteru2/core/engine/factory"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// AddNode adds a node
// node with resource info
func (c *Calcium) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	logger := log.WithField("Calcium", "AddNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	var resourceCapacity map[string]types.NodeResourceArgs
	var resourceUsage map[string]types.NodeResourceArgs
	var node *types.Node
	var err error

	// check if the node is alive
	client, err := enginefactory.GetEngine(ctx, c.config, opts.Nodename, opts.Endpoint, opts.Ca, opts.Cert, opts.Key)
	if err != nil {
		return nil, err
	}
	// get node info
	nodeInfo, err := client.Info(ctx)
	if err != nil {
		return nil, err
	}

	return node, utils.Txn(
		ctx,
		// if: add node resource with resource plugins
		func(ctx context.Context) error {
			resourceCapacity, resourceUsage, err = c.rmgr.AddNode(ctx, opts.Nodename, opts.ResourceOpts, nodeInfo)
			return err
		},
		// then: add node meta in store
		func(ctx context.Context) error {
			node, err = c.store.AddNode(ctx, opts)
			if err != nil {
				return err
			}
			node.Resource.Capacity = resourceCapacity
			node.Resource.Usage = resourceUsage
			_ = c.pool.Invoke(func() { c.doSendNodeMetrics(ctx, node) })
			return nil
		},
		// rollback: remove node with resource plugins
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			return c.rmgr.RemoveNode(ctx, opts.Nodename)
		},
		c.config.GlobalTimeout)
}

// RemoveNode remove a node
func (c *Calcium) RemoveNode(ctx context.Context, nodename string) error {
	logger := log.WithField("Calcium", "RemoveNode").WithField("nodename", nodename)
	if nodename == "" {
		logger.Error(ctx, types.ErrEmptyNodeName)
		return types.ErrEmptyNodeName
	}
	return c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
		workloads, err := c.ListNodeWorkloads(ctx, node.Name, nil)
		if err != nil {
			logger.Error(ctx, err)
			return err
		}
		// need drain first
		if len(workloads) > 0 {
			logger.Error(ctx, types.ErrNodeNotEmpty)
			return types.ErrNodeNotEmpty
		}

		return utils.Txn(ctx,
			// if: remove node metadata
			func(ctx context.Context) error {
				return c.store.RemoveNode(ctx, node)
			},
			// then: remove node resource metadata
			func(ctx context.Context) error {
				return c.rmgr.RemoveNode(ctx, nodename)
			},
			// rollback: do nothing
			func(ctx context.Context, failureByCond bool) error {
				return nil
			},
			c.config.GlobalTimeout)
	})
}

// ListPodNodes list nodes belong to pod
// node with resource info
func (c *Calcium) ListPodNodes(ctx context.Context, opts *types.ListNodesOptions) (<-chan *types.Node, error) {
	logger := log.WithField("Calcium", "ListPodNodes").WithField("podname", opts.Podname).WithField("labels", opts.Labels).WithField("all", opts.All).WithField("info", opts.CallInfo)
	nodes, err := c.store.GetNodesByPod(ctx, &types.NodeFilter{Podname: opts.Podname, Labels: opts.Labels, All: opts.All})
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	ch := make(chan *types.Node)

	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(nodes))
		defer wg.Wait()
		for _, node := range nodes {
			node := node
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				var err error
				if node.Resource.Capacity, node.Resource.Usage, node.Resource.Diffs, err = c.rmgr.GetNodeResourceInfo(ctx, node.Name, nil, false); err != nil {
					logger.Errorf(ctx, err, "failed to get node %v resource info: %+v", node.Name, err)
				}
				if opts.CallInfo {
					if err := node.Info(ctx); err != nil {
						logger.Errorf(ctx, err, "failed to get node %v info: %+v", node.Name, err)
					}
				}
				ch <- node
			})
		}
	})

	return ch, nil
}

// GetNode get node
// node with resource info
func (c *Calcium) GetNode(ctx context.Context, nodename string) (node *types.Node, err error) {
	logger := log.WithField("Calcium", "GetNode").WithField("nodename", nodename)
	if nodename == "" {
		logger.Error(ctx, types.ErrEmptyNodeName)
		return nil, types.ErrEmptyNodeName
	}
	if node, err = c.store.GetNode(ctx, nodename); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	if node.Resource.Capacity, node.Resource.Usage, node.Resource.Diffs, err = c.rmgr.GetNodeResourceInfo(ctx, node.Name, nil, false); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	return node, nil
}

// GetNodeEngineInfo get node engine
func (c *Calcium) GetNodeEngineInfo(ctx context.Context, nodename string) (*enginetypes.Info, error) {
	logger := log.WithField("Calcium", "GetNodeEngine").WithField("nodename", nodename)
	if nodename == "" {
		logger.Error(ctx, types.ErrEmptyNodeName)
		return nil, types.ErrEmptyNodeName
	}
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	engineInfo, err := node.Engine.Info(ctx)
	logger.Error(ctx, err)
	return engineInfo, err
}

// SetNode set node available or not
// node with resource info
func (c *Calcium) SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error) {
	logger := log.WithField("Calcium", "SetNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	var n *types.Node
	return n, c.withNodePodLocked(ctx, opts.Nodename, func(ctx context.Context, node *types.Node) error {
		logger.Infof(ctx, "set node")
		// update resource map
		var err error
		node.Resource.Capacity, node.Resource.Usage, node.Resource.Diffs, err = c.rmgr.GetNodeResourceInfo(ctx, node.Name, nil, false)
		if err != nil {
			return err
		}
		n = node

		n.Bypass = (opts.Bypass == types.TriTrue) || (opts.Bypass == types.TriKeep && n.Bypass)
		if n.IsDown() {
			logger.Errorf(ctx, err, "[SetNode] node marked down: %s", opts.Nodename)
		}

		if opts.WorkloadsDown {
			c.setAllWorkloadsOnNodeDown(ctx, n.Name)
		}

		// update node endpoint
		if opts.Endpoint != "" {
			n.Endpoint = opts.Endpoint
		}
		// update ca / cert / key
		n.Ca = opts.Ca
		n.Cert = opts.Cert
		n.Key = opts.Key
		// update key value
		if len(opts.Labels) != 0 {
			n.Labels = opts.Labels
		}

		var originNodeResourceCapacity map[string]types.NodeResourceArgs
		return utils.Txn(ctx,
			// if: update node resource capacity success
			func(ctx context.Context) error {
				if len(opts.ResourceOpts) == 0 {
					return nil
				}

				originNodeResourceCapacity, _, err = c.rmgr.SetNodeResourceCapacity(ctx, n.Name, opts.ResourceOpts, nil, opts.Delta, resources.Incr)
				return err
			},
			// then: update node metadata
			func(ctx context.Context) error {
				if err := c.store.UpdateNodes(ctx, n); err != nil {
					return err
				}
				// update resource
				// actually we can ignore err here, if update success
				if len(opts.ResourceOpts) != 0 {
					n.Resource.Capacity, n.Resource.Usage, n.Resource.Diffs, _ = c.rmgr.GetNodeResourceInfo(ctx, node.Name, nil, false)
				}
				// use send to update the usage
				_ = c.pool.Invoke(func() { c.doSendNodeMetrics(ctx, n) })
				return nil
			},
			// rollback: update node resource capacity in reverse
			func(ctx context.Context, failureByCond bool) error {
				if failureByCond {
					return nil
				}
				if len(opts.ResourceOpts) == 0 {
					return nil
				}
				_, _, err = c.rmgr.SetNodeResourceCapacity(ctx, n.Name, nil, originNodeResourceCapacity, false, resources.Decr)
				return err
			},
			c.config.GlobalTimeout)
	})
}

// filterNodes filters nodes using NodeFilter nf
// the filtering logic is introduced along with NodeFilter
// NOTE: when nf.Includes is set, they don't need to belong to podname
// update on 2021-06-21: sort and unique locks to avoid deadlock
// node without resource info if batch get
func (c *Calcium) filterNodes(ctx context.Context, nodeFilter *types.NodeFilter) (ns []*types.Node, err error) {
	defer func() {
		if len(ns) == 0 {
			return
		}
		// sorted by nodenames
		nodenames := utils.Map(ns, func(node *types.Node) string { return node.Name })
		// unique
		p := utils.Unique(nodenames, func(i int) string { return nodenames[i] })
		ns = ns[:p]
	}()

	if len(nodeFilter.Includes) != 0 {
		for _, nodename := range nodeFilter.Includes {
			node, err := c.store.GetNode(ctx, nodename)
			if err != nil {
				return nil, err
			}
			ns = append(ns, node)
		}
		return ns, nil
	}

	listedNodes, err := c.store.GetNodesByPod(ctx, nodeFilter)
	if err != nil {
		return nil, err
	}
	if len(nodeFilter.Excludes) == 0 {
		return listedNodes, nil
	}

	excludes := map[string]struct{}{}
	for _, n := range nodeFilter.Excludes {
		excludes[n] = struct{}{}
	}

	for _, n := range listedNodes {
		if _, ok := excludes[n.Name]; ok {
			continue
		}
		ns = append(ns, n)
	}
	return ns, nil
}

func (c *Calcium) setAllWorkloadsOnNodeDown(ctx context.Context, nodename string) {
	workloads, err := c.store.ListNodeWorkloads(ctx, nodename, nil)
	if err != nil {
		log.Errorf(ctx, err, "[setAllWorkloadsOnNodeDown] failed to list node workloads, node %v, err: %v", nodename, err)
		return
	}

	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			log.Errorf(ctx, err, "[setAllWorkloadsOnNodeDown] Set workload %s on node %s as inactive failed %v", workload.ID, nodename, err)
			continue
		}

		if workload.StatusMeta == nil {
			workload.StatusMeta = &types.StatusMeta{ID: workload.ID}
		}
		workload.StatusMeta.Running = false
		workload.StatusMeta.Healthy = false

		// Set these attributes to set workload status
		workload.StatusMeta.Appname = appname
		workload.StatusMeta.Nodename = workload.Nodename
		workload.StatusMeta.Entrypoint = entrypoint

		// mark workload which belongs to this node as unhealthy
		if err = c.store.SetWorkloadStatus(ctx, workload.StatusMeta, 0); err != nil {
			log.Errorf(ctx, err, "[SetNodeAvailable] Set workload %s on node %s as inactive failed %v", workload.ID, nodename, err)
		} else {
			log.Infof(ctx, "[SetNodeAvailable] Set workload %s on node %s as inactive", workload.ID, nodename)
		}
	}
}
