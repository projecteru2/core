package store

import (
	"context"
	"time"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

const (
	// ActionIncr for incr resource
	ActionIncr = "+"
	// ActionDecr for decr resource
	ActionDecr = "-"
)

// Store store eru data
type Store interface {
	// service
	ServiceStatusStream(context.Context) (chan []string, error)
	RegisterService(context.Context, string, time.Duration) (<-chan struct{}, func(), error)

	// ephemeral nodes primitive
	StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error)

	// pod
	AddPod(ctx context.Context, name, desc string) (*types.Pod, error)
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	RemovePod(ctx context.Context, podname string) error
	GetAllPods(ctx context.Context) ([]*types.Pod, error)

	// node
	AddNode(context.Context, *types.AddNodeOptions) (*types.Node, error)
	RemoveNode(ctx context.Context, node *types.Node) error
	GetNode(ctx context.Context, nodename string) (*types.Node, error)
	GetNodes(ctx context.Context, nodenames []string) ([]*types.Node, error)
	GetNodesByPod(ctx context.Context, podname string, labels map[string]string, all bool) ([]*types.Node, error)
	UpdateNodes(context.Context, ...*types.Node) error
	UpdateNodeResource(ctx context.Context, node *types.Node, resource *types.ResourceMeta, action string) error
	SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error
	NodeStatusStream(ctx context.Context) chan *types.NodeStatus

	// workload
	AddWorkload(ctx context.Context, workload *types.Workload) error
	UpdateWorkload(ctx context.Context, workload *types.Workload) error
	RemoveWorkload(ctx context.Context, workload *types.Workload) error
	GetWorkload(ctx context.Context, ID string) (*types.Workload, error)
	GetWorkloads(ctx context.Context, IDs []string) ([]*types.Workload, error)
	GetWorkloadStatus(ctx context.Context, ID string) (*types.StatusMeta, error)
	SetWorkloadStatus(ctx context.Context, workload *types.Workload, ttl int64) error
	ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error)
	ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error)
	WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus

	// deploy status
	MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, strategyInfo []strategy.Info) error

	// processing status
	SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error
	UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error
	DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodename string) error

	// distributed lock
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)

	// embedded storage
	TerminateEmbededStorage()
}
