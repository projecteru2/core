package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"

	"github.com/projecteru2/core/utils"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// AddPod add pod
func (c *Calcium) AddPod(ctx context.Context, podname, desc string) (*types.Pod, error) {
	return c.store.AddPod(ctx, podname, desc)
}

// AddNode add a node in pod
func (c *Calcium) AddNode(ctx context.Context, nodename, endpoint, podname, ca, cert, key string,
	cpu, share int, memory, storage int64, labels map[string]string,
	numa types.NUMA, numaMemory types.NUMAMemory) (*types.Node, error) {
	return c.store.AddNode(ctx, nodename, endpoint, podname, ca, cert, key, cpu, share, memory, storage, labels, numa, numaMemory)
}

// RemovePod remove pod
func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	return c.withNodesLocked(ctx, podname, "", nil, func(nodes map[string]*types.Node) error {
		// TODO dissociate container to node
		// remove node first
		return c.store.RemovePod(ctx, podname)
	})
}

// RemoveNode remove a node
func (c *Calcium) RemoveNode(ctx context.Context, podname, nodename string) error {
	return c.withNodeLocked(ctx, podname, nodename, func(node *types.Node) error {
		return c.store.DeleteNode(ctx, node)
	})
}

// ListPods show pods
func (c *Calcium) ListPods(ctx context.Context) ([]*types.Pod, error) {
	return c.store.GetAllPods(ctx)
}

// ListPodNodes list nodes belong to pod
func (c *Calcium) ListPodNodes(ctx context.Context, podname string, all bool) ([]*types.Node, error) {
	var nodes []*types.Node
	candidates, err := c.store.GetNodesByPod(ctx, podname)
	if err != nil {
		log.Errorf("[ListPodNodes] Error during ListPodNodes from %s: %v", podname, err)
		return nodes, err
	}
	for _, candidate := range candidates {
		if candidate.Available || all {
			nodes = append(nodes, candidate)
		}
	}
	return nodes, nil
}

// ListContainers list containers
func (c *Calcium) ListContainers(ctx context.Context, opts *types.ListContainersOptions) ([]*types.Container, error) {
	return c.store.ListContainers(ctx, opts.Appname, opts.Entrypoint, opts.Nodename)
}

// ListNodeContainers list containers belong to one node
func (c *Calcium) ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error) {
	return c.store.ListNodeContainers(ctx, nodename)
}

// GetPod get one pod
func (c *Calcium) GetPod(ctx context.Context, podname string) (*types.Pod, error) {
	return c.store.GetPod(ctx, podname)
}

// GetNode get node
func (c *Calcium) GetNode(ctx context.Context, podname, nodename string) (*types.Node, error) {
	return c.store.GetNode(ctx, podname, nodename)
}

// GetContainer get a container
func (c *Calcium) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	return c.store.GetContainer(ctx, ID)
}

// GetContainers get containers
func (c *Calcium) GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error) {
	return c.store.GetContainers(ctx, IDs)
}

// SetNode set node available or not
func (c *Calcium) SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error) {
	var n *types.Node
	return n, c.withNodeLocked(ctx, opts.Podname, opts.Nodename, func(node *types.Node) error {
		n = node
		n.Available = n.Available || opts.Available
		if !n.Available {
			containers, err := c.store.ListNodeContainers(ctx, opts.Nodename)
			if err != nil {
				return err
			}
			for _, container := range containers {
				appname, entrypoint, _, err := utils.ParseContainerName(container.Name)
				if err != nil {
					log.Errorf("[SetNodeAvailable] Get container %s on node %s failed %v", container.ID, opts.Nodename, err)
					continue
				}
				// mark container which belongs to this node as unhealthy
				if err := c.ContainerDeployed(ctx, container.ID, appname, entrypoint, container.Nodename, []byte{}, 0); err != nil {
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
		// update storage
		n.StorageCap += opts.DeltaStorage
		n.InitStorageCap += opts.DeltaStorage
		if n.StorageCap < 0 {
			return types.ErrBadStorage
		}
		// update memory
		n.MemCap += opts.DeltaMemory
		n.InitMemCap += opts.DeltaMemory
		if n.MemCap < 0 {
			return types.ErrBadStorage
		}
		// update cpu
		for cpuID, cpuShare := range opts.DeltaCPU {
			if _, ok := n.CPU[cpuID]; !ok && cpuShare > 0 { // 增加了 CPU
				n.CPU[cpuID] = cpuShare
				n.InitCPU[cpuID] = cpuShare
			} else if ok && cpuShare == 0 { // 删掉 CPU
				delete(n.CPU, cpuID)
				delete(n.InitCPU, cpuID)
			} else if ok { // 减少份数
				n.CPU[cpuID] += cpuShare
				n.InitCPU[cpuID] += cpuShare
				if n.CPU[cpuID] < 0 {
					return types.ErrBadCPU
				}
			}
		}
		return c.store.UpdateNode(ctx, n)
	})
}

// GetNodeByName get node by name
func (c *Calcium) GetNodeByName(ctx context.Context, nodename string) (*types.Node, error) {
	return c.store.GetNodeByName(ctx, nodename)
}

// ContainerDeployed set container deploy status
func (c *Calcium) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename string, data []byte, ttl int64) error {
	return c.store.ContainerDeployed(ctx, ID, appname, entrypoint, nodename, data, ttl)
}
