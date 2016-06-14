package store

import "gitlab.ricebook.net/platform/core/types"

type Store interface {
	// pod
	AddPod(name, desc string) (*types.Pod, error)
	GetPod(podname string) (*types.Pod, error)
	GetAllPod() ([]*types.Pod, error)

	// node
	AddNode(name, endpoint, podname string, public bool) (*types.Node, error)
	GetNode(podname, nodename string) (*types.Node, error)
	GetNodesByPod(podname string) ([]*types.Node, error)
	GetAllNodes() ([]*types.Node, error)
	UpdateNode(*types.Node) error

	// container
	AddContainer(id, podname, nodename string) (*types.Container, error)
	GetContainer(id string) (*types.Container, error)
	RemoveContainer(id string) error
}
