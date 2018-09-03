package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// AddPod add pod
func (c *Calcium) AddPod(ctx context.Context, podname, favor, desc string) (*types.Pod, error) {
	return c.store.AddPod(ctx, podname, favor, desc)
}

// AddNode add a node in pod
func (c *Calcium) AddNode(ctx context.Context, nodename, endpoint, podname, ca, cert, key string, cpu, share int, memory int64, labels map[string]string) (*types.Node, error) {
	return c.store.AddNode(ctx, nodename, endpoint, podname, ca, cert, key, cpu, share, memory, labels)
}

// RemovePod remove pod
func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	return c.store.RemovePod(ctx, podname)
}

// RemoveNode remove a node
func (c *Calcium) RemoveNode(ctx context.Context, nodename, podname string) (*types.Pod, error) {
	n, err := c.GetNode(ctx, podname, nodename)
	if err != nil {
		return nil, err
	}
	c.store.DeleteNode(ctx, n)
	return c.store.GetPod(ctx, podname)
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
		log.Debugf("[ListPodNodes] Error during ListPodNodes from %s: %v", podname, err)
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

// SetNodeAvailable set node available or not
func (c *Calcium) SetNodeAvailable(ctx context.Context, podname, nodename string, available bool) (*types.Node, error) {
	n, err := c.GetNode(ctx, podname, nodename)
	if err != nil {
		return nil, err
	}
	n.Available = available
	if err := c.store.UpdateNode(ctx, n); err != nil {
		return nil, err
	}
	return n, nil
}

// GetNodeByName get node by name
func (c *Calcium) GetNodeByName(ctx context.Context, nodename string) (*types.Node, error) {
	return c.store.GetNodeByName(ctx, nodename)
}

// ContainerDeployed show container deploy status
func (c *Calcium) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error {
	return c.store.ContainerDeployed(ctx, ID, appname, entrypoint, nodename, data)
}
