package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"context"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
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

// CleanPod clean pod images
func (c *Calcium) CleanPod(ctx context.Context, podname string, prune bool, images []string) error {
	nodes, err := c.store.GetNodesByPod(ctx, podname)
	if err != nil {
		log.Debugf("[CleanPod] Error during GetNodesByPod %s %v", podname, err)
		return err
	}
	wg := sync.WaitGroup{}
	for _, node := range nodes {
		wg.Add(1)
		go func(node *types.Node) {
			defer wg.Done()
			for _, image := range images {
				if _, err := node.Engine.ImageRemove(ctx, image, enginetypes.ImageRemoveOptions{PruneChildren: true}); err != nil {
					log.Infof("[CleanPod] Clean %s pod %s node %s image", podname, node.Name, image)
				} else {
					log.Errorf("[CleanPod] Clean %s pod %s node %s image failed: %v", podname, node.Name, image, err)
				}
			}
			if prune {
				_, err := node.Engine.ImagesPrune(ctx, filters.NewArgs())
				if err != nil {
					log.Errorf("[CleanPod] Prune %s pod %s node failed: %v", podname, node.Name, err)
				}
			}
		}(node)
	}
	wg.Wait()
	return nil
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
func (c *Calcium) ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error) {
	return c.store.ListContainers(ctx, appname, entrypoint, nodename)
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
