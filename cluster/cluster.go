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
	// NodeUp for node up
	NodeUp = 1
	// NodeDown for node down
	NodeDown = 0
	// KeepNodeStatus for no change node status
	KeepNodeStatus = 2
)

// Cluster define all interface
type Cluster interface {
	// meta networks
	ListNetworks(ctx context.Context, podname string, driver string) ([]*enginetypes.Network, error)
	// meta pod
	ListPods(ctx context.Context) ([]*types.Pod, error)
	AddPod(ctx context.Context, podname, desc string) (*types.Pod, error)
	RemovePod(ctx context.Context, podname string) error
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	PodResource(ctx context.Context, podname string) (*types.PodResource, error)
	ListPodNodes(ctx context.Context, podname string, all bool) ([]*types.Node, error)
	// meta node
	AddNode(ctx context.Context, nodename, endpoint, podname, ca, cert, key string,
		cpu, share int, memory, storage int64, labels map[string]string,
		numa types.NUMA, numaMemory types.NUMAMemory) (*types.Node, error)
	RemoveNode(ctx context.Context, podname, nodename string) error
	SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error)
	GetNode(ctx context.Context, podname, nodename string) (*types.Node, error)
	NodeResource(ctx context.Context, podname, nodename string) (*types.NodeResource, error)
	GetNodeByName(ctx context.Context, nodename string) (*types.Node, error)
	// meta containers
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	ListContainers(ctx context.Context, opts *types.ListContainersOptions) ([]*types.Container, error)
	ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error)
	SetContainersStatus(ctx context.Context, status map[string][]byte, ttls map[string]int64) error
	// cluster methods
	Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error)
	Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error)
	// image methods
	BuildImage(ctx context.Context, opts *enginetypes.BuildOptions) (chan *types.BuildImageMessage, error)
	CacheImage(ctx context.Context, podname, nodenmae string, images []string, step int) (chan *types.CacheImageMessage, error)
	RemoveImage(ctx context.Context, podname, nodename string, images []string, step int, prune bool) (chan *types.RemoveImageMessage, error)
	// for watch
	DeployStatusStream(ctx context.Context, appname, entrypoint, nodename string) chan *types.DeployStatus
	// container methods
	CreateContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateContainerMessage, error)
	ReplaceContainer(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceContainerMessage, error)
	RemoveContainer(ctx context.Context, IDs []string, force bool, step int) (chan *types.RemoveContainerMessage, error)
	DissociateContainer(ctx context.Context, IDs []string) (chan *types.DissociateContainerMessage, error)
	ControlContainer(ctx context.Context, IDs []string, t string, force bool) (chan *types.ControlContainerMessage, error)
	ReallocResource(ctx context.Context, IDs []string, cpu float64, memory int64) (chan *types.ReallocResourceMessage, error)
	LogStream(ctx context.Context, ID string) (chan *types.LogStreamMessage, error)
	RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) (<-chan *types.AttachContainerMessage, error)
	ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions, inCh <-chan []byte) chan *types.AttachContainerMessage
	// finalizer
	Finalizer()
}
