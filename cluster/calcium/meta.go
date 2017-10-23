package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	log "github.com/Sirupsen/logrus"
	"github.com/projecteru2/core/types"
)

func (c *calcium) ListPods() ([]*types.Pod, error) {
	return c.store.GetAllPods()
}

func (c *calcium) AddPod(podname, favor, desc string) (*types.Pod, error) {
	return c.store.AddPod(podname, favor, desc)
}

func (c *calcium) RemovePod(podname string) error {
	return c.store.RemovePod(podname)
}

func (c *calcium) GetPod(podname string) (*types.Pod, error) {
	return c.store.GetPod(podname)
}

func (c *calcium) AddNode(nodename, endpoint, podname, ca, cert, key string, public bool, cpu int, share, memory int64) (*types.Node, error) {
	return c.store.AddNode(nodename, endpoint, podname, ca, cert, key, public, cpu, share, memory)
}

func (c *calcium) GetNode(podname, nodename string) (*types.Node, error) {
	return c.store.GetNode(podname, nodename)
}

func (c *calcium) GetNodeByName(nodename string) (*types.Node, error) {
	return c.store.GetNodeByName(nodename)
}

func (c *calcium) SetNodeAvailable(podname, nodename string, available bool) (*types.Node, error) {
	n, err := c.store.GetNode(podname, nodename)
	if err != nil {
		return nil, err
	}
	n.Available = available
	if err := c.store.UpdateNode(n); err != nil {
		return nil, err
	}
	return n, nil
}

func (c *calcium) RemoveNode(nodename, podname string) (*types.Pod, error) {
	n, err := c.store.GetNode(podname, nodename)
	if err != nil {
		return nil, err
	}
	c.store.DeleteNode(n)
	return c.store.GetPod(podname)
}

func (c *calcium) ListPodNodes(podname string, all bool) ([]*types.Node, error) {
	var nodes []*types.Node
	candidates, err := c.store.GetNodesByPod(podname)
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

func (c *calcium) GetContainer(id string) (*types.Container, error) {
	return c.store.GetContainer(id)
}

func (c *calcium) GetContainers(ids []string) ([]*types.Container, error) {
	return c.store.GetContainers(ids)
}

func (c *calcium) ContainerDeployed(ID, appname, entrypoint, nodename, data string) error {
	return c.store.ContainerDeployed(ID, appname, entrypoint, nodename, data)
}
