package cluster

import (
	"context"

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
	// ERUMark mark container controlled by eru
	ERUMark = "ERU"
	// LabelMeta store publish and health things
	LabelMeta = "ERU_META"
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
	// meta service
	WatchServiceStatus(context.Context) (<-chan types.ServiceStatus, error)
	// meta networks
	ListNetworks(ctx context.Context, podname string, driver string) ([]*enginetypes.Network, error)
	ConnectNetwork(ctx context.Context, network, target, ipv4, ipv6 string) ([]string, error)
	DisconnectNetwork(ctx context.Context, network, target string, force bool) error
	// meta pod
	AddPod(ctx context.Context, podname, desc string) (*types.Pod, error)
	RemovePod(ctx context.Context, podname string) error
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	ListPods(ctx context.Context) ([]*types.Pod, error)
	// pod resource
	PodResource(ctx context.Context, podname string) (*types.PodResource, error)
	// meta node
	AddNode(context.Context, *types.AddNodeOptions) (*types.Node, error)
	RemoveNode(ctx context.Context, nodename string) error
	ListPodNodes(ctx context.Context, podname string, labels map[string]string, all bool) ([]*types.Node, error)
	GetNode(ctx context.Context, nodename string) (*types.Node, error)
	SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error)
	// node resource
	NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error)
	// meta containers
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	ListContainers(ctx context.Context, opts *types.ListContainersOptions) ([]*types.Container, error)
	ListNodeContainers(ctx context.Context, nodename string, labels map[string]string) ([]*types.Container, error)
	GetContainersStatus(ctx context.Context, IDs []string) ([]*types.StatusMeta, error)
	SetContainersStatus(ctx context.Context, status []*types.StatusMeta, ttls map[string]int64) ([]*types.StatusMeta, error)
	ContainerStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.ContainerStatus
	// file methods
	Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error)
	Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error)
	// image methods
	BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error)
	CacheImage(ctx context.Context, podname string, nodenames []string, images []string, step int) (chan *types.CacheImageMessage, error)
	RemoveImage(ctx context.Context, podname string, nodenames []string, images []string, step int, prune bool) (chan *types.RemoveImageMessage, error)
	// container methods
	CalculateCapacity(context.Context, *types.CalculateCapacityOptions) (*types.CapacityMessage, error)
	CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	ReplaceContainer(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceContainerMessage, error)
	RemoveContainer(ctx context.Context, IDs []string, force bool, step int) (chan *types.RemoveContainerMessage, error)
	DissociateContainer(ctx context.Context, IDs []string) (chan *types.DissociateContainerMessage, error)
	ControlContainer(ctx context.Context, IDs []string, t string, force bool) (chan *types.ControlContainerMessage, error)
	ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions, inCh <-chan []byte) chan *types.AttachContainerMessage
	ReallocResource(ctx context.Context, opts *types.ReallocOptions) (chan *types.ReallocResourceMessage, error)
	LogStream(ctx context.Context, opts *types.LogStreamOptions) (chan *types.LogStreamMessage, error)
	RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) (<-chan *types.AttachContainerMessage, error)
	// finalizer
	Finalizer()
}
