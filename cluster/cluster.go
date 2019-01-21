package cluster

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

const (
	// Gitlab for gitlab
	Gitlab = "gitlab"
	// Github for github
	Github = "github"
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
	// DeployGlobal for global node resource plan
	DeployGlobal = "global"
	// ERUMark mark container controlled by eru
	ERUMark = "ERU"
	// ERUMeta store publish and health things
	ERUMeta = "ERU_META"
	// ContainerStop for stop container
	ContainerStop = "stop"
	// ContainerStart for start container
	ContainerStart = "start"
	// ContainerRestart for restart container
	ContainerRestart = "restart"
	// ContainerLock for lock container
	ContainerLock = "clock_%s"
	// NodeLock for lock node
	NodeLock = "cnode_%s_%s"
)

// Cluster define all interface
type Cluster interface {
	// meta data methods
	AddPod(ctx context.Context, podname, favor, desc string) (*types.Pod, error)
	AddNode(ctx context.Context, nodename, endpoint, podname, ca, cert, key string, cpu, share int, memory int64, labels map[string]string) (*types.Node, error)
	RemovePod(ctx context.Context, podname string) error
	RemoveNode(ctx context.Context, nodename, podname string) (*types.Pod, error)
	ListPods(ctx context.Context) ([]*types.Pod, error)
	ListPodNodes(ctx context.Context, podname string, all bool) ([]*types.Node, error)
	ListContainers(ctx context.Context, opts *types.ListContainersOptions) ([]*types.Container, error)
	ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error)
	ListNetworks(ctx context.Context, podname string, driver string) ([]*enginetypes.Network, error)
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	GetNode(ctx context.Context, podname, nodename string) (*types.Node, error)
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	SetNodeAvailable(ctx context.Context, podname, nodename string, available bool) (*types.Node, error)
	// used by agent
	GetNodeByName(ctx context.Context, nodename string) (*types.Node, error)
	ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error

	// cluster methods
	PodResource(ctx context.Context, podname string) (*types.PodResource, error)
	ControlContainer(ctx context.Context, IDs []string, t string) (chan *types.ControlContainerMessage, error)
	Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error)
	RemoveImage(ctx context.Context, podname, nodename string, images []string, prune bool) (chan *types.RemoveImageMessage, error)
	RunAndWait(ctx context.Context, opts *types.DeployOptions, stdin io.ReadCloser) (chan *types.RunAndWaitMessage, error)
	DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus
	// build methods
	BuildImage(ctx context.Context, opts *enginetypes.BuildOptions) (chan *types.BuildImageMessage, error)
	// this methods will not interrupt by client
	CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	ReallocResource(ctx context.Context, IDs []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error)
	RemoveContainer(ctx context.Context, IDs []string, force bool) (chan *types.RemoveContainerMessage, error)
	ReplaceContainer(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceContainerMessage, error)
}
