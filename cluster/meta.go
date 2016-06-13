package cluster

import "gitlab.ricebook.net/platform/core/types"

// All functions are just proxy to store,
// since I don't want store to be exported.
// All these functions are meta data related.
func (c *Calcium) ListPods() ([]types.Pod, error) {
	return c.store.GetAllPods()
}

func (c *Calcium) ListPods() ([]types.Pod, error) {
	return c.store.GetAllPods()
}

func (c *Calcium) AddPod(podname, desc string) (types.Pod, error) {
	return c.store.AddPod(podname, desc)
}

func (c *Calcium) GetPod(podname string) (types.Pod, error) {
	return c.store.GetPod(podname)
}

func (c *Calcium) AddNode(nodename, endpoint, podname string, public bool) (types.Node, error) {
	return c.store.AddNode(nodename, endpoint, podname, public)
}

func (c *Calcium) GetNode(podname, nodename string) (types.Node, error) {
	return c.store.GetNode(podname, nodename)
}

func (c *Calcium) ListPodNodes(podname string) ([]types.Node, error) {
	return c.store.GetNodesByPod(podname)
}
