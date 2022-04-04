package calcium

import (
	"context"
	"sort"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	enginetypes "github.com/projecteru2/core/engine/types"

	"github.com/pkg/errors"
)

// AddNode adds a node
func (c *Calcium) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	logger := log.WithField("Calcium", "AddNode").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(ctx, err)
	}
	opts.Normalize()
	node, err := c.store.AddNode(ctx, opts)
	return node, logger.Err(ctx, errors.WithStack(err))
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
		return logger.Err(ctx, errors.WithStack(c.store.RemoveNode(ctx, node)))
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
					err := node.Info(ctx)
					if err != nil {
						logger.Errorf(ctx, "failed to get node %v info: %+v", node.Name, err)
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
func (c *Calcium) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	logger := log.WithField("Calcium", "GetNode").WithField("nodename", nodename)
	if nodename == "" {
		return nil, logger.Err(ctx, errors.WithStack(types.ErrEmptyNodeName))
	}
	node, err := c.store.GetNode(ctx, nodename)
	return node, logger.Err(ctx, errors.WithStack(err))
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
		opts.Normalize(node)
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
		// update numa
		if len(opts.NUMA) != 0 {
			n.NUMA = opts.NUMA
		}
		// update numa memory
		for numaNode, memoryDelta := range opts.DeltaNUMAMemory {
			if _, ok := n.NUMAMemory[numaNode]; ok {
				n.NUMAMemory[numaNode] += memoryDelta
				n.InitNUMAMemory[numaNode] += memoryDelta
				if n.NUMAMemory[numaNode] < 0 {
					return logger.Err(ctx, errors.WithStack(types.ErrBadMemory))
				}
			}
		}
		if opts.DeltaStorage != 0 {
			// update storage
			n.StorageCap += opts.DeltaStorage
			n.InitStorageCap += opts.DeltaStorage
			if n.StorageCap < 0 {
				return logger.Err(ctx, errors.WithStack(types.ErrBadStorage))
			}
		}
		if opts.DeltaMemory != 0 {
			// update memory
			n.MemCap += opts.DeltaMemory
			n.InitMemCap += opts.DeltaMemory
			if n.MemCap < 0 {
				return logger.Err(ctx, errors.WithStack(types.ErrBadStorage))
			}
		}
		// update cpu
		for cpuID, cpuShare := range opts.DeltaCPU {
			_, ok := n.CPU[cpuID]
			switch {
			case !ok && cpuShare > 0: // incr CPU
				n.CPU[cpuID] = cpuShare
				n.InitCPU[cpuID] = cpuShare
			case ok: // decr share
				n.CPU[cpuID] += cpuShare
				n.InitCPU[cpuID] += cpuShare
				if n.CPU[cpuID] < 0 {
					return logger.Err(ctx, errors.WithStack(types.ErrBadCPU))
				}
				if n.InitCPU[cpuID] == 0 {
					// decr CPU
					delete(n.CPU, cpuID)
					delete(n.InitCPU, cpuID)
				}
			}
		}
		// update volume
		for volumeDir, changeCap := range opts.DeltaVolume {
			_, ok := n.Volume[volumeDir]
			switch {
			case !ok && changeCap > 0:
				n.Volume[volumeDir] = changeCap
				n.InitVolume[volumeDir] = changeCap
			case ok && changeCap == 0:
				delete(n.Volume, volumeDir)
				delete(n.InitVolume, volumeDir)
			case ok:
				n.Volume[volumeDir] += changeCap
				n.InitVolume[volumeDir] += changeCap
				if n.Volume[volumeDir] < 0 {
					return logger.Err(ctx, errors.WithStack(types.ErrBadVolume))
				}
			}
		}
		return logger.Err(ctx, errors.WithStack(c.store.UpdateNodes(ctx, n)))
	})
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
