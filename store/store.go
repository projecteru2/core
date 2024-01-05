package store

import (
	"context"
	"time"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
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
	GetNodesByPod(ctx context.Context, nodeFilter *types.NodeFilter, opts ...Option) ([]*types.Node, error)
	UpdateNodes(context.Context, ...*types.Node) error
	SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error
	GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error)
	NodeStatusStream(ctx context.Context) chan *types.NodeStatus
	LoadNodeCert(ctx context.Context, node *types.Node) (err error)

	// workload
	AddWorkload(context.Context, *types.Workload, *types.Processing) error
	UpdateWorkload(ctx context.Context, workload *types.Workload) error
	RemoveWorkload(ctx context.Context, workload *types.Workload) error
	GetWorkload(ctx context.Context, ID string) (*types.Workload, error)
	GetWorkloads(ctx context.Context, IDs []string) ([]*types.Workload, error)
	GetWorkloadStatus(ctx context.Context, ID string) (*types.StatusMeta, error)
	SetWorkloadStatus(ctx context.Context, status *types.StatusMeta, ttl int64) error
	ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error)
	ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error)
	WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus

	// deploy status
	GetDeployStatus(ctx context.Context, appname, entryname string) (map[string]int, error)

	// processing status
	CreateProcessing(ctx context.Context, process *types.Processing, count int) error
	DeleteProcessing(context.Context, *types.Processing) error

	// distributed lock
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)
}
