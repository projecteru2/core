package calcium

import (
	"context"
	"sort"

	"github.com/pkg/errors"

	enginefactory "github.com/projecteru2/core/engine/factory"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// AddNode adds a node
func (c *Calcium) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	logger := log.WithField("Calcium", "AddNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(ctx, err)
	}
	var resourceCapacity map[string]types.NodeResourceArgs
	var resourceUsage map[string]types.NodeResourceArgs
	var node *types.Node
	var err error

	// check if the node is alive
	client, err := enginefactory.GetEngine(ctx, c.config, opts.Nodename, opts.Endpoint, opts.Ca, opts.Cert, opts.Key)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	// get node info
	nodeInfo, err := client.Info(ctx)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return node, logger.Err(ctx, utils.Txn(
		ctx,
		// if: add node resource with resource plugins
		func(ctx context.Context) error {
			resourceCapacity, resourceUsage, err = c.resource.AddNode(ctx, opts.Nodename, opts.ResourceOpts, nodeInfo)
			return errors.WithStack(err)
		},
		// then: add node meta in store
		func(ctx context.Context) error {
			node, err = c.store.AddNode(ctx, opts)
			if err != nil {
				return errors.WithStack(err)
			}
			node.ResourceCapacity = resourceCapacity
			node.ResourceUsage = resourceUsage
			go c.SendNodeMetrics(ctx, node.Name)
			return nil
		},
		// rollback: remove node with resource plugins
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			return errors.WithStack(c.resource.RemoveNode(ctx, opts.Nodename))
		},
		c.config.GlobalTimeout),
	)
}

// RemoveNode remove a node
func (c *Calcium) RemoveNode(ctx context.Context, nodename string) error {
	logger := log.WithField("Calcium", "RemoveNode").WithField("nodename", nodename)
	if nodename == "" {
		return logger.Err(ctx, errors.WithStack(types.ErrEmptyNodeName))
	}
	return c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
		ws, err := c.ListNodeWorkloads(ctx, node.Name, nil)
		if err != nil {
			return logger.Err(ctx, err)
		}
		if len(ws) > 0 {
			return logger.Err(ctx, errors.WithStack(types.ErrNodeNotEmpty))
		}

		return logger.Err(ctx, utils.Txn(ctx,
			// if: remove node metadata
			func(ctx context.Context) error {
				return errors.WithStack(c.store.RemoveNode(ctx, node))
			},
			// then: remove node resource metadata
			func(ctx context.Context) error {
				return errors.WithStack(c.resource.RemoveNode(ctx, nodename))
			},
			// rollback: do nothing
			func(ctx context.Context, failureByCond bool) error {
				return nil
			},
			c.config.GlobalTimeout,
		))
	})
}

// ListPodNodes list nodes belong to pod
func (c *Calcium) ListPodNodes(ctx context.Context, opts *types.ListNodesOptions) (<-chan *types.Node, error) {
	logger := log.WithField("Calcium", "ListPodNodes").WithField("podname", opts.Podname).WithField("labels", opts.Labels).WithField("all", opts.All).WithField("info", opts.Info)
	ch := make(chan *types.Node)
	nodes, err := c.store.GetNodesByPod(ctx, opts.Podname, opts.Labels, opts.All)
	if err != nil || !opts.Info {
		go func() {
			defer close(ch)
			for _, node := range nodes {
				if err := c.getNodeResourceInfo(ctx, node); err != nil {
					logger.Errorf(ctx, "failed to get node %v resource info: %+v", node.Name, err)
				}
				ch <- node
			}
		}()
		return ch, logger.Err(ctx, errors.WithStack(err))
	}

	pool := utils.NewGoroutinePool(int(c.config.MaxConcurrency))
	go func() {
		defer close(ch)
		for _, node := range nodes {
			pool.Go(ctx, func(node *types.Node) func() {
				return func() {
					if err := node.Info(ctx); err != nil {
						logger.Errorf(ctx, "failed to get node %v info: %+v", node.Name, err)
					}
					if err := c.getNodeResourceInfo(ctx, node); err != nil {
						logger.Errorf(ctx, "failed to get node %v resource info: %+v", node.Name, err)
					}
					ch <- node
				}
			}(node))
		}
		pool.Wait(ctx)
	}()
	return ch, nil
}

// GetNode get node
func (c *Calcium) GetNode(ctx context.Context, nodename string) (node *types.Node, err error) {
	logger := log.WithField("Calcium", "GetNode").WithField("nodename", nodename)
	if nodename == "" {
		return nil, logger.Err(ctx, errors.WithStack(types.ErrEmptyNodeName))
	}
	if node, err = c.store.GetNode(ctx, nodename); err != nil {
		return nil, logger.Err(ctx, errors.WithStack(err))
	}
	if err = c.getNodeResourceInfo(ctx, node); err != nil {
		return nil, logger.Err(ctx, errors.WithStack(err))
	}
	return node, nil
}

// GetNodeEngine get node engine
func (c *Calcium) GetNodeEngine(ctx context.Context, nodename string) (*enginetypes.Info, error) {
	logger := log.WithField("Calcium", "GetNodeEngine").WithField("nodename", nodename)
	if nodename == "" {
		return nil, logger.Err(ctx, errors.WithStack(types.ErrEmptyNodeName))
	}
	node, err := c.store.GetNode(ctx, nodename)
	if err != nil {
		return nil, logger.Err(ctx, errors.WithStack(err))
	}
	engineInfo, err := node.Engine.Info(ctx)
	return engineInfo, logger.Err(ctx, errors.WithStack(err))
}

// SetNode set node available or not
func (c *Calcium) SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error) {
	logger := log.WithField("Calcium", "SetNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(ctx, err)
	}
	var n *types.Node
	return n, c.withNodePodLocked(ctx, opts.Nodename, func(ctx context.Context, node *types.Node) error {
		logger.Infof(ctx, "set node")
		n = node

		n.Bypass = (opts.BypassOpt == types.TriTrue) || (opts.BypassOpt == types.TriKeep && n.Bypass)
		if n.IsDown() {
			logger.Errorf(ctx, "[SetNodeAvailable] node marked down: %s", opts.Nodename)
		}
		if opts.WorkloadsDown {
			c.setAllWorkloadsOnNodeDown(ctx, opts.Nodename)
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
		var err error

		return logger.Err(ctx, utils.Txn(ctx,
			// if: update node resource capacity success
			func(ctx context.Context) error {
				if len(opts.ResourceOpts) == 0 {
					return nil
				}

				originNodeResourceCapacity, _, err = c.resource.SetNodeResourceCapacity(ctx, n.Name, opts.ResourceOpts, nil, opts.Delta, resources.Incr)
				return errors.WithStack(err)
			},
			// then: update node metadata
			func(ctx context.Context) error {
				if err := errors.WithStack(c.store.UpdateNodes(ctx, n)); err != nil {
					return err
				}
				go c.SendNodeMetrics(ctx, node.Name)
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
				_, _, err = c.resource.SetNodeResourceCapacity(ctx, n.Name, nil, originNodeResourceCapacity, false, resources.Decr)
				return errors.WithStack(err)
			},
			c.config.GlobalTimeout,
		))
	})
}

func (c *Calcium) getNodeResourceInfo(ctx context.Context, node *types.Node) (err error) {
	if node.ResourceCapacity, node.ResourceUsage, _, err = c.resource.GetNodeResourceInfo(ctx, node.Name, nil, false); err != nil {
		log.Errorf(ctx, "[getNodeResourceInfo] failed to get node resource info for node %v, err: %v", node.Name, err)
		return errors.WithStack(err)
	}
	return nil
}

func (c *Calcium) setAllWorkloadsOnNodeDown(ctx context.Context, nodename string) {
	workloads, err := c.store.ListNodeWorkloads(ctx, nodename, nil)
	if err != nil {
		log.Errorf(ctx, "[setAllWorkloadsOnNodeDown] failed to list node workloads, node %v, err: %v", nodename, errors.WithStack(err))
		return
	}

	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			log.Errorf(ctx, "[setAllWorkloadsOnNodeDown] Set workload %s on node %s as inactive failed %v", workload.ID, nodename, err)
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
			log.Errorf(ctx, "[SetNodeAvailable] Set workload %s on node %s as inactive failed %v", workload.ID, nodename, errors.WithStack(err))
		} else {
			log.Infof(ctx, "[SetNodeAvailable] Set workload %s on node %s as inactive", workload.ID, nodename)
		}
	}
}

// filterNodes filters nodes using NodeFilter nf
// the filtering logic is introduced along with NodeFilter
// NOTE: when nf.Includes is set, they don't need to belong to podname
// updateon 2021-06-21: sort and unique locks to avoid deadlock
func (c *Calcium) filterNodes(ctx context.Context, nf types.NodeFilter) (ns []*types.Node, err error) {
	defer func() {
		if len(ns) == 0 {
			return
		}
		sort.Slice(ns, func(i, j int) bool { return ns[i].Name <= ns[j].Name })
		// unique
		ns = ns[:utils.Unique(ns, func(i int) string { return ns[i].Name })]
	}()

	if len(nf.Includes) != 0 {
		for _, nodename := range nf.Includes {
			node, err := c.GetNode(ctx, nodename)
			if err != nil {
				return nil, err
			}
			ns = append(ns, node)
		}
		return ns, nil
	}

	ch, err := c.ListPodNodes(ctx, &types.ListNodesOptions{
		Podname: nf.Podname,
		Labels:  nf.Labels,
		All:     nf.All,
	})
	if err != nil {
		return nil, err
	}
	listedNodes := []*types.Node{}
	for n := range ch {
		listedNodes = append(listedNodes, n)
	}
	if len(nf.Excludes) == 0 {
		return listedNodes, nil
	}

	excludes := map[string]struct{}{}
	for _, n := range nf.Excludes {
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
