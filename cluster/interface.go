package cluster

import (
	"gitlab.ricebook.net/platform/core/types"
)

type Cluster interface {
	// meta data methods
	ListPods() ([]*types.Pod, error)
	AddPod(podname, desc string) (*types.Pod, error)
	GetPod(podname string) (*types.Pod, error)
	AddNode(nodename, endpoint, podname string, public bool) (*types.Node, error)
	GetNode(podname, nodename string) (*types.Node, error)
	ListPodNodes(podname string) ([]*types.Node, error)
	GetContainer(id string) (*types.Container, error)

	// cluster methods
	BuildImage() error
	CreateContainer() error
	UpdateContainer() error
	RemoveContainer() error
	MigrateContainer() error
}
