package calcium

import (
	"cmp"
	"context"
	"slices"

	"github.com/cockroachdb/errors"

	enginefactory "github.com/projecteru2/core/engine/factory"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/resource/plugins"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	logger := log.WithFunc("calcium.AddNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	var res resourcetypes.Resources
	var node *types.Node
	var err error

	client, err := enginefactory.GetEngine(ctx, c.config, opts.Nodename, opts.Endpoint, opts.Ca, opts.Cert, opts.Key)
	if err != nil {
		return nil, err
	}
	nodeInfo, err := client.Info(ctx)
	if err != nil {
		return nil, err
	}

	_, txnErr := utils.Txn(
		ctx,
		func(ctx context.Context) error {
			res, err = c.rmgr.AddNode(ctx, opts.Nodename, opts.Resources, nodeInfo)
			if !errors.Is(err, types.ErrNodeExists) {
				return err
			}
			_, getErr := c.store.GetNode(ctx, opts.Nodename)
			if getErr == nil || !c.store.NotFound(getErr) {
				return err
			}
			logger.Warn(ctx, "node known to the resource plugins only, dropping the stale metadata")
			if err = c.rmgr.RemoveNode(ctx, opts.Nodename); err != nil {
				return err
			}
			res, err = c.rmgr.AddNode(ctx, opts.Nodename, opts.Resources, nodeInfo)
			return err
		},
		func(ctx context.Context) error {
			node, err = c.store.AddNode(ctx, opts)
			if err != nil {
				return err
			}
			node.ResourceInfo.Capacity = res
			_ = c.pool.Invoke(func() { c.doSendNodeMetrics(utils.NewInheritCtx(ctx), node) })
			return nil
		},
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			return c.rmgr.RemoveNode(ctx, opts.Nodename)
		},
		c.config.GlobalTimeout,
	)
	return node, txnErr
}

func (c *Calcium) RemoveNode(ctx context.Context, nodename string) error {
	logger := log.WithFunc("calcium.RemoveNode").WithField("node", nodename)
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
		if len(workloads) > 0 {
			logger.Error(ctx, types.ErrNodeNotEmpty)
			return types.ErrNodeNotEmpty
		}

		_, txnErr := utils.Txn(ctx,
			func(ctx context.Context) error {
				// a down node has no status key, so peers miss the removal unless one is written first
				if err = c.store.SetNodeStatus(ctx, node, 90); err != nil {
					logger.Warnf(ctx, "failed to set node status: %s", err)
				}
				if err := c.store.RemoveNode(ctx, node); err != nil {
					return err
				}
				// ttl -1 deletes the status key
				_ = c.store.SetNodeStatus(ctx, node, -1)
				return nil
			},
			func(ctx context.Context) error {
				if err := c.rmgr.RemoveNode(ctx, nodename); err != nil {
					return err
				}
				enginefactory.RemoveEngineFromCache(ctx, node.Endpoint, node.Ca, node.Cert, node.Key)
				metrics.Client.RemoveInvalidNodes(nodename)
				return nil
			},
			func(_ context.Context, _ bool) error {
				return nil
			},
			c.config.GlobalTimeout)
		return txnErr
	})
}

func (c *Calcium) ListPodNodes(ctx context.Context, opts *types.ListNodesOptions) (<-chan *types.Node, error) {
	logger := log.WithFunc("calcium.ListPodNodes").WithField("podname", opts.Podname).WithField("labels", opts.Labels).WithField("all", opts.All).WithField("info", opts.CallInfo)
	nf := &types.NodeFilter{Podname: opts.Podname, Labels: opts.Labels, All: opts.All}
	nodes, err := c.store.GetNodesByPod(ctx, nf, !opts.CallInfo)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	return perNode(c, nodes, func(node *types.Node, ch chan<- *types.Node) {
		if err := c.refreshResourceInfo(ctx, node); err != nil {
			logger.Errorf(ctx, err, "failed to get node %s resource info", node.Name)
		}
		if opts.CallInfo {
			if err := node.Info(ctx); err != nil {
				logger.Errorf(ctx, err, "failed to get node %s info", node.Name)
			}
		}
		ch <- node
	}), nil
}

func (c *Calcium) GetNode(ctx context.Context, nodename string) (node *types.Node, err error) {
	logger := log.WithFunc("calcium.GetNode").WithField("node", nodename)
	if nodename == "" {
		logger.Error(ctx, types.ErrEmptyNodeName)
		return nil, types.ErrEmptyNodeName
	}
	if node, err = c.store.GetNode(ctx, nodename); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	if err = c.refreshResourceInfo(ctx, node); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	return node, nil
}

func (c *Calcium) GetNodeEngineInfo(ctx context.Context, nodename string) (*enginetypes.Info, error) {
	logger := log.WithFunc("calcium.GetNodeEngineInfo").WithField("node", nodename)
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

func (c *Calcium) SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error) {
	logger := log.WithFunc("calcium.SetNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	var n *types.Node
	return n, c.withNodePodLocked(ctx, opts.Nodename, func(ctx context.Context, node *types.Node) error {
		logger.Info(ctx, "set node")
		var err error
		if err = c.refreshResourceInfo(ctx, node); err != nil {
			return err
		}
		n = node

		n.Bypass = (opts.Bypass == types.TriTrue) || (opts.Bypass == types.TriKeep && n.Bypass)
		if n.IsDown() {
			logger.Warnf(ctx, "node marked down: %s", opts.Nodename)
		}

		if opts.WorkloadsDown {
			c.setAllWorkloadsOnNodeDown(ctx, n.Name)
		}

		if opts.Endpoint != "" {
			n.Endpoint = opts.Endpoint
		}
		if opts.UpdateTLS {
			n.Ca = opts.Ca
			n.Cert = opts.Cert
			n.Key = opts.Key
		}
		if len(opts.Labels) != 0 {
			n.Labels = opts.Labels
		}

		var origin resourcetypes.Resources
		_, txnErr := utils.Txn(ctx,
			func(ctx context.Context) error {
				if len(opts.Resources) == 0 {
					return nil
				}
				origin, _, err = c.rmgr.SetNodeResourceCapacity(ctx, n.Name, nil, opts.Resources, opts.Delta, plugins.Incr)
				return err
			},
			func(ctx context.Context) error {
				defer enginefactory.RemoveEngineFromCache(ctx, node.Endpoint, node.Ca, node.Cert, node.Key)
				if updateErr := c.store.UpdateNodes(ctx, n); updateErr != nil {
					return updateErr
				}
				// capacity refresh is best effort; the store write already succeeded
				_ = c.refreshResourceInfo(ctx, n)
				_ = c.pool.Invoke(func() { c.doSendNodeMetrics(utils.NewInheritCtx(ctx), n) })
				_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })
				return nil
			},
			func(ctx context.Context, failureByCond bool) error {
				if failureByCond {
					return nil
				}
				if len(opts.Resources) == 0 {
					return nil
				}
				_, _, err = c.rmgr.SetNodeResourceCapacity(ctx, n.Name, nil, origin, false, plugins.Decr)
				return err
			},
			c.config.GlobalTimeout)
		return txnErr
	})
}

func (c *Calcium) filterNodes(ctx context.Context, nodeFilter *types.NodeFilter) (ns []*types.Node, err error) {
	defer func() {
		ns = slices.SortedFunc(slices.Values(ns), func(a, b *types.Node) int { return cmp.Compare(a.Name, b.Name) })
		ns = slices.CompactFunc(ns, func(a, b *types.Node) bool { return a.Name == b.Name })
	}()

	if len(nodeFilter.Includes) != 0 {
		for _, nodename := range nodeFilter.Includes {
			node, getErr := c.store.GetNode(ctx, nodename)
			if getErr != nil {
				return nil, getErr
			}
			ns = append(ns, node)
		}
		return ns, nil
	}

	listedNodes, err := c.store.GetNodesByPod(ctx, nodeFilter, false)
	if err != nil {
		return nil, err
	}
	if len(nodeFilter.Excludes) == 0 {
		return listedNodes, nil
	}

	return slices.DeleteFunc(listedNodes, func(n *types.Node) bool {
		return slices.Contains(nodeFilter.Excludes, n.Name)
	}), nil
}

// refreshResourceInfo fills the node's capacity, usage and diffs from the resource plugins.
func (c *Calcium) refreshResourceInfo(ctx context.Context, node *types.Node) error {
	var err error
	node.ResourceInfo.Capacity, node.ResourceInfo.Usage, node.ResourceInfo.Diffs, err = c.rmgr.GetNodeResourceInfo(ctx, node.Name, nil, false)
	return err
}

func (c *Calcium) setAllWorkloadsOnNodeDown(ctx context.Context, nodename string) {
	workloads, err := c.store.ListNodeWorkloads(ctx, nodename, nil)
	logger := log.WithFunc("calcium.setAllWorkloadsOnNodeDown").WithField("node", nodename)
	if err != nil {
		logger.Errorf(ctx, err, "failed to list node workloads, node %s", nodename)
		return
	}

	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			logger.Errorf(ctx, err, "set workload %s on node %s as inactive failed", workload.ID, nodename)
			continue
		}

		if workload.StatusMeta == nil {
			workload.StatusMeta = &types.StatusMeta{ID: workload.ID}
		}
		workload.StatusMeta.Running = false
		workload.StatusMeta.Healthy = false

		workload.StatusMeta.Appname = appname
		workload.StatusMeta.Nodename = workload.Nodename
		workload.StatusMeta.Entrypoint = entrypoint

		if err = c.store.SetWorkloadStatus(ctx, workload.StatusMeta, 0); err != nil {
			logger.Errorf(ctx, err, "set workload %s on node %s as inactive failed", workload.ID, nodename)
		} else {
			logger.Infof(ctx, "set workload %s on node %s as inactive", workload.ID, nodename)
		}
	}
}
