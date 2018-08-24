package cluster

import (
	"context"
	"io"

	"github.com/projecteru2/core/types"
)

const (
	// GITLAB for gitlab
	GITLAB = "gitlab"
	// GITHUB for github
	GITHUB = "github"
	// CopyFailed for copy failed
	CopyFailed = "failed"
	// CopyOK for copy ok
	CopyOK = "ok"
	// CPUPeriodBase for cpu period base
	CPUPeriodBase = 100000
	// DeployAuto for auto deploy plan
	DeployAuto = "auto"
	// DeployEach for each node plan
	DeployEach = "each"
	// DeployFill for fill node plan
	DeployFill = "fill"
)

// Cluster define all interface
type Cluster interface {
	// meta data methods
	AddPod(ctx context.Context, podname, favor, desc string) (*types.Pod, error)
	AddNode(ctx context.Context, nodename, endpoint, podname, ca, cert, key string, cpu, share int, memory int64, labels map[string]string) (*types.Node, error)
	RemovePod(ctx context.Context, podname string) error
	CleanPod(ctx context.Context, podname string, prune bool, images []string) error
	RemoveNode(ctx context.Context, nodename, podname string) (*types.Pod, error)
	ListPods(ctx context.Context) ([]*types.Pod, error)
	ListPodNodes(ctx context.Context, podname string, all bool) ([]*types.Node, error)
	ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error)
	ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error)
	ListNetworks(ctx context.Context, podname string, driver string) ([]*types.Network, error)
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	GetNode(ctx context.Context, podname, nodename string) (*types.Node, error)
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	SetNodeAvailable(ctx context.Context, podname, nodename string, available bool) (*types.Node, error)

	// cluster methods
	Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error)
	BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error)
	RemoveImage(ctx context.Context, podname, nodename string, images []string) (chan *types.RemoveImageMessage, error)
	DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus
	RunAndWait(ctx context.Context, opts *types.DeployOptions, stdin io.ReadCloser) (chan *types.RunAndWaitMessage, error)
	// this methods will not interrupt by client
	CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	ReplaceContainer(ctx context.Context, opts *types.DeployOptions, force bool) (chan *types.ReplaceContainerMessage, error)
	RemoveContainer(ctx context.Context, IDs []string, force bool) (chan *types.RemoveContainerMessage, error)
	ReallocResource(ctx context.Context, IDs []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error)

	// used by agent
	GetNodeByName(ctx context.Context, nodename string) (*types.Node, error)
	ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error
}
