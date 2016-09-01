package cluster

import (
	"gitlab.ricebook.net/platform/core/types"
)

type Cluster interface {
	// meta data methods
	ListPods() ([]*types.Pod, error)
	AddPod(podname, desc string) (*types.Pod, error)
	GetPod(podname string) (*types.Pod, error)
	AddNode(nodename, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error)
	GetNode(podname, nodename string) (*types.Node, error)
	ListPodNodes(podname string) ([]*types.Node, error)
	ListPodNodeNames(podname string) ([]string, error)
	GetContainer(id string) (*types.Container, error)
	GetContainers(ids []string) ([]*types.Container, error)

	// cluster methods
	BuildImage(repository, version, uid, artifact string) (chan *types.BuildImageMessage, error)
	CreateContainer(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	UpgradeContainer(ids []string, image string) (chan *types.UpgradeContainerMessage, error)
	RemoveContainer(ids []string) (chan *types.RemoveContainerMessage, error)
	RemoveImage(podname, nodename string, images []string) (chan *types.RemoveImageMessage, error)
}
