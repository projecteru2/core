package cluster

import (
	"io"

	"gitlab.ricebook.net/platform/core/types"
)

type Cluster interface {
	// meta data methods
	ListPods() ([]*types.Pod, error)
	AddPod(podname, desc string) (*types.Pod, error)
	GetPod(podname string) (*types.Pod, error)
	AddNode(nodename, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error)
	RemoveNode(nodename, podname string) (*types.Pod, error)
	GetNode(podname, nodename string) (*types.Node, error)
	SetNodeAvailable(podname, nodename string, available bool) (*types.Node, error)
	ListPodNodes(podname string, all bool) ([]*types.Node, error)
	GetContainer(id string) (*types.Container, error)
	GetContainers(ids []string) ([]*types.Container, error)
	ListNetworks(podname string) ([]*types.Network, error)

	// cluster methods
	BuildImage(repository, version, uid, artifact string) (chan *types.BuildImageMessage, error)
	CreateContainer(specs types.Specs, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	RunAndWait(specs types.Specs, opts *types.DeployOptions, stdin io.ReadCloser) (chan *types.RunAndWaitMessage, error)
	RemoveContainer(ids []string) (chan *types.RemoveContainerMessage, error)
	RemoveImage(podname, nodename string, images []string) (chan *types.RemoveImageMessage, error)
	Backup(id, srcPath string) (*types.BackupMessage, error)

	// cluster attribute methods
	GetZone() string
}
