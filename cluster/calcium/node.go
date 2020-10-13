package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
	"github.com/sanity-io/litter"
	log "github.com/sirupsen/logrus"
)

// AddNode adds a node
func (c *Calcium) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	opts.Normalize()
	return c.store.AddNode(ctx, opts)
}

// RemoveNode remove a node
func (c *Calcium) RemoveNode(ctx context.Context, nodename string) error {
	return c.withNodeLocked(ctx, nodename, func(node *types.Node) error {
		return c.store.RemoveNode(ctx, node)
	})
}

// ListPodNodes list nodes belong to pod
func (c *Calcium) ListPodNodes(ctx context.Context, podname string, labels map[string]string, all bool) ([]*types.Node, error) {
	return c.store.GetNodesByPod(ctx, podname, labels, all)
}

// GetNode get node
func (c *Calcium) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	return c.store.GetNode(ctx, nodename)
}

// SetNode set node available or not
func (c *Calcium) SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error) { // nolint
	var n *types.Node
	return n, c.withNodeLocked(ctx, opts.Nodename, func(node *types.Node) error {
		litter.Dump(opts)
		opts.Normalize(node)
		n = node
		n.Available = (opts.Status == types.TriTrue) || (opts.Status == types.TriKeep && n.Available)
		if opts.ContainersDown {
			containers, err := c.store.ListNodeContainers(ctx, opts.Nodename, nil)
			if err != nil {
				return err
			}
			for _, container := range containers {
				if container.StatusMeta == nil {
					container.StatusMeta = &types.StatusMeta{ID: container.ID}
				}
				container.StatusMeta.Running = false
				container.StatusMeta.Healthy = false

				// mark container which belongs to this node as unhealthy
				if err = c.store.SetContainerStatus(ctx, container, 0); err != nil {
					log.Errorf("[SetNodeAvailable] Set container %s on node %s inactive failed %v", container.ID, opts.Nodename, err)
				}
			}
		}
		// update key value
		if len(opts.Labels) != 0 {
			n.Labels = opts.Labels
		}
		// update numa
		if len(opts.NUMA) != 0 {
			n.NUMA = types.NUMA(opts.NUMA)
		}
		// update numa memory
		for numaNode, memoryDelta := range opts.DeltaNUMAMemory {
			if _, ok := n.NUMAMemory[numaNode]; ok {
				n.NUMAMemory[numaNode] += memoryDelta
				n.InitNUMAMemory[numaNode] += memoryDelta
				if n.NUMAMemory[numaNode] < 0 {
					return types.ErrBadMemory
				}
			}
		}
		if opts.DeltaStorage != 0 {
			// update storage
			n.StorageCap += opts.DeltaStorage
			n.InitStorageCap += opts.DeltaStorage
			if n.StorageCap < 0 {
				return types.ErrBadStorage
			}
		}
		if opts.DeltaMemory != 0 {
			// update memory
			n.MemCap += opts.DeltaMemory
			n.InitMemCap += opts.DeltaMemory
			if n.MemCap < 0 {
				return types.ErrBadStorage
			}
		}
		// update cpu
		for cpuID, cpuShare := range opts.DeltaCPU {
			_, ok := n.CPU[cpuID]
			switch {
			case !ok && cpuShare > 0: // incr CPU
				n.CPU[cpuID] = cpuShare
				n.InitCPU[cpuID] = cpuShare
			case ok && cpuShare == 0: // decr CPU
				delete(n.CPU, cpuID)
				delete(n.InitCPU, cpuID)
			case ok: // decr share
				n.CPU[cpuID] += cpuShare
				n.InitCPU[cpuID] += cpuShare
				if n.CPU[cpuID] < 0 {
					return types.ErrBadCPU
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
					return types.ErrBadVolume
				}
			}
		}
		return c.store.UpdateNodes(ctx, n)
	})
}

// GetNodes get nodes
func (c *Calcium) getNodes(ctx context.Context, podname string, nodenames []string, labels map[string]string, all bool) ([]*types.Node, error) {
	var err error
	ns := []*types.Node{}
	if len(nodenames) != 0 {
		for _, nodename := range nodenames {
			node, err := c.GetNode(ctx, nodename)
			if err != nil {
				return ns, err
			}
			ns = append(ns, node)
		}
	} else {
		ns, err = c.ListPodNodes(ctx, podname, labels, all)
	}
	return ns, err
}
