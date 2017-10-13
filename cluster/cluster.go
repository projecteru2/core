package cluster

import (
	"io"

	"github.com/projecteru2/core/types"
)

type Cluster interface {
	// meta data methods
	ListPods() ([]*types.Pod, error)
	AddPod(podname, favor, desc string) (*types.Pod, error)
	RemovePod(podname string) error
	GetPod(podname string) (*types.Pod, error)
	AddNode(nodename, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error)
	RemoveNode(nodename, podname string) (*types.Pod, error)
	GetNode(podname, nodename string) (*types.Node, error)
	SetNodeAvailable(podname, nodename string, available bool) (*types.Node, error)
	ListPodNodes(podname string, all bool) ([]*types.Node, error)
	GetContainer(id string) (*types.Container, error)
	GetContainers(ids []string) ([]*types.Container, error)
	ListNetworks(podname string, driver string) ([]*types.Network, error)

	// cluster methods
	BuildImage(opts *types.BuildOptions) (chan *types.BuildImageMessage, error)
	CreateContainer(opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	RunAndWait(opts *types.DeployOptions, timeout int, stdin io.ReadCloser) (chan *types.RunAndWaitMessage, error)
	RemoveContainer(ids []string, force bool) (chan *types.RemoveContainerMessage, error)
	RemoveImage(podname, nodename string, images []string) (chan *types.RemoveImageMessage, error)
	Backup(id, srcPath string) (*types.BackupMessage, error)
	ReallocResource(ids []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error)

	// used by agent
	GetNodeByName(nodename string) (*types.Node, error)
	ContainerDeployed(ID, appname, entrypoint, nodename, data string) error
}
