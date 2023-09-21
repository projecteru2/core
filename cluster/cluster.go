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
	// CPUPeriodBase for cpu period base
	CPUPeriodBase = 100000
	// ERUMark mark workload controlled by eru
	ERUMark = "ERU"
	// LabelMeta store publish and health things
	LabelMeta = "ERU_META"
	// LabelCoreID is used to label a container with its identifier
	LabelCoreID = "eru.coreid"
	// LabelNodeName is used to label a container with the nodename
	LabelNodeName = "eru.nodename"
	// WorkloadStop for stop workload
	WorkloadStop = "stop"
	// WorkloadStart for start workload
	WorkloadStart = "start"
	// WorkloadRestart for restart workload
	WorkloadRestart = "restart"
	// WorkloadSuspend for suspending workload
	WorkloadSuspend = "suspend"
	// WorkloadResume for resuming workload
	WorkloadResume = "resume"
	// WorkloadLock for lock workload
	WorkloadLock = "clock_%s"
	// PodLock for lock pod
	PodLock = "plock_%s"
	// NodeOperationLock for lock node operation
	NodeOperationLock = "cnode_op_%s_%s"
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
	PodResource(ctx context.Context, podname string) (chan *types.NodeResourceInfo, error)
	// meta node
	AddNode(context.Context, *types.AddNodeOptions) (*types.Node, error)
	RemoveNode(ctx context.Context, nodename string) error
	ListPodNodes(context.Context, *types.ListNodesOptions) (<-chan *types.Node, error)
	GetNode(ctx context.Context, nodename string) (*types.Node, error)
	GetNodeEngineInfo(ctx context.Context, nodename string) (*enginetypes.Info, error)
	SetNode(ctx context.Context, opts *types.SetNodeOptions) (*types.Node, error)
	SetNodeStatus(ctx context.Context, nodename string, ttl int64) error
	GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error)
	NodeStatusStream(ctx context.Context) chan *types.NodeStatus
	// node resource
	NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResourceInfo, error)
	// calculate capacity
	CalculateCapacity(context.Context, *types.DeployOptions) (*types.CapacityMessage, error)
	// meta workloads
	GetWorkload(ctx context.Context, id string) (*types.Workload, error)
	GetWorkloads(ctx context.Context, IDs []string) ([]*types.Workload, error)
	ListWorkloads(ctx context.Context, opts *types.ListWorkloadsOptions) ([]*types.Workload, error)
	ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error)
	GetWorkloadsStatus(ctx context.Context, IDs []string) ([]*types.StatusMeta, error)
	SetWorkloadsStatus(ctx context.Context, status []*types.StatusMeta, ttls map[string]int64) ([]*types.StatusMeta, error)
	WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus
	// file methods
	Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error)
	Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error)
	SendLargeFile(ctx context.Context, opts chan *types.SendLargeFileOptions) chan *types.SendMessage
	// image methods
	BuildImage(ctx context.Context, opts *types.BuildOptions) (chan *types.BuildImageMessage, error)
	CacheImage(ctx context.Context, opts *types.ImageOptions) (chan *types.CacheImageMessage, error)
	RemoveImage(ctx context.Context, opts *types.ImageOptions) (chan *types.RemoveImageMessage, error)
	ListImage(ctx context.Context, opts *types.ImageOptions) (chan *types.ListImageMessage, error)
	// workload methods
	CreateWorkload(ctx context.Context, opts *types.DeployOptions) (chan *types.CreateWorkloadMessage, error)
	ReplaceWorkload(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceWorkloadMessage, error)
	RemoveWorkload(ctx context.Context, IDs []string, force bool) (chan *types.RemoveWorkloadMessage, error)
	RemoveWorkloadSync(ctx context.Context, IDs []string) error
	DissociateWorkload(ctx context.Context, IDs []string) (chan *types.DissociateWorkloadMessage, error)
	ControlWorkload(ctx context.Context, IDs []string, t string, force bool) (chan *types.ControlWorkloadMessage, error)
	ExecuteWorkload(ctx context.Context, opts *types.ExecuteWorkloadOptions, inCh <-chan []byte) chan *types.AttachWorkloadMessage
	ReallocResource(ctx context.Context, opts *types.ReallocOptions) error
	LogStream(ctx context.Context, opts *types.LogStreamOptions) (chan *types.LogStreamMessage, error)
	RunAndWait(ctx context.Context, opts *types.DeployOptions, inCh <-chan []byte) ([]string, <-chan *types.AttachWorkloadMessage, error)
	RawEngine(ctx context.Context, opts *types.RawEngineOptions) (*types.RawEngineMessage, error)
	// finalizer
	Finalizer()

	// GetIdentifier returns the identifier for this cluster
	// identifier will be used to label a container, to announce the container belongs to this cluster
	GetIdentifier() string
}
