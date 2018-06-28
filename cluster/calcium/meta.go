package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import (
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

//ListPods show pods
func (c *Calcium) ListPods() ([]*types.Pod, error) {
	return c.store.GetAllPods()
}

//AddPod add pod
func (c *Calcium) AddPod(podname, favor, desc string) (*types.Pod, error) {
	return c.store.AddPod(podname, favor, desc)
}

//RemovePod remove pod
func (c *Calcium) RemovePod(podname string) error {
	return c.store.RemovePod(podname)
}

//GetPod get one pod
func (c *Calcium) GetPod(podname string) (*types.Pod, error) {
	return c.store.GetPod(podname)
}

//AddNode add a node in pod
func (c *Calcium) AddNode(nodename, endpoint, podname, ca, cert, key string, cpu int, share, memory int64, labels map[string]string) (*types.Node, error) {
	return c.store.AddNode(nodename, endpoint, podname, ca, cert, key, cpu, share, memory, labels)
}

//GetNode get node
func (c *Calcium) GetNode(podname, nodename string) (*types.Node, error) {
	return c.store.GetNode(podname, nodename)
}

//GetNodeByName get node by name
func (c *Calcium) GetNodeByName(nodename string) (*types.Node, error) {
	return c.store.GetNodeByName(nodename)
}

//SetNodeAvailable set node available or not
func (c *Calcium) SetNodeAvailable(podname, nodename string, available bool) (*types.Node, error) {
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

//RemoveNode remove a node
func (c *Calcium) RemoveNode(nodename, podname string) (*types.Pod, error) {
	n, err := c.store.GetNode(podname, nodename)
	if err != nil {
		return nil, err
	}
	c.store.DeleteNode(n)
	return c.store.GetPod(podname)
}

//ListPodNodes list nodes belong to pod
func (c *Calcium) ListPodNodes(podname string, all bool) ([]*types.Node, error) {
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

//GetContainer get a container
func (c *Calcium) GetContainer(id string) (*types.Container, error) {
	return c.store.GetContainer(id)
}

//GetContainers get containers
func (c *Calcium) GetContainers(ids []string) ([]*types.Container, error) {
	return c.store.GetContainers(ids)
}

//ContainerDeployed show container deploy status
func (c *Calcium) ContainerDeployed(ID, appname, entrypoint, nodename, data string) error {
	return c.store.ContainerDeployed(ID, appname, entrypoint, nodename, data)
}

//ListContainers list containers
func (c *Calcium) ListContainers(appname, entrypoint, nodename string) ([]*types.Container, error) {
	return c.store.ListContainers(appname, entrypoint, nodename)
}
