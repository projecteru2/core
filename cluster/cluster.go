package cluster

import (
	"context"
	"io"

	"github.com/projecteru2/core/types"
)

type Cluster interface {
	// meta data methods
	ListPods() ([]*types.Pod, error)
	AddPod(podname, favor, desc string) (*types.Pod, error)
	RemovePod(podname string) error
	GetPod(podname string) (*types.Pod, error)
	AddNode(nodename, endpoint, podname, ca, cert, key string, public bool, cpu int, share, memory int64) (*types.Node, error)
	RemoveNode(nodename, podname string) (*types.Pod, error)
	GetNode(podname, nodename string) (*types.Node, error)
	SetNodeAvailable(podname, nodename string, available bool) (*types.Node, error)
	ListPodNodes(podname string, all bool) ([]*types.Node, error)
	GetContainer(id string) (*types.Container, error)
	GetContainers(ids []string) ([]*types.Container, error)
	ListNetworks(podname string, driver string) ([]*types.Network, error)

	// cluster methods
	BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error)
	RemoveImage(ctx context.Context, podname, nodename string, images []string) (chan *types.RemoveImageMessage, error)
	DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus
	RunAndWait(ctx context.Context, opts *types.DeployOptions, stdin io.ReadCloser) (chan *types.RunAndWaitMessage, error)
	CreateContainer(opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	RemoveContainer(ids []string, force bool) (chan *types.RemoveContainerMessage, error)
	Backup(id, srcPath string) (*types.BackupMessage, error)
	ReallocResource(ids []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error)

	// used by agent
	GetNodeByName(nodename string) (*types.Node, error)
	ContainerDeployed(ID, appname, entrypoint, nodename, data string) error
}
