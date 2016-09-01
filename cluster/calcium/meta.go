package calcium

// All functions are just proxy to store, since I don't want store to be exported.
// All these functions are meta data related.

import "gitlab.ricebook.net/platform/core/types"

func (c *calcium) ListPods() ([]*types.Pod, error) {
	return c.store.GetAllPods()
}

func (c *calcium) AddPod(podname, desc string) (*types.Pod, error) {
	return c.store.AddPod(podname, desc)
}

func (c *calcium) GetPod(podname string) (*types.Pod, error) {
	return c.store.GetPod(podname)
}

func (c *calcium) AddNode(nodename, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error) {
	return c.store.AddNode(nodename, endpoint, podname, cafile, certfile, keyfile, public)
}

func (c *calcium) GetNode(podname, nodename string) (*types.Node, error) {
	return c.store.GetNode(podname, nodename)
}

func (c *calcium) ListPodNodes(podname string) ([]*types.Node, error) {
	return c.store.GetNodesByPod(podname)
}

func (c *calcium) ListPodNodeNames(podname string) ([]string, error) {
	return c.store.GetNodeNamesByPod(podname)
}

func (c *calcium) GetContainer(id string) (*types.Container, error) {
	return c.store.GetContainer(id)
}

func (c *calcium) GetContainers(ids []string) ([]*types.Container, error) {
	return c.store.GetContainers(ids)
}
