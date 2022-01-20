package store

import (
	"context"
	"testing"
	"time"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/store/etcdv3"
	"github.com/projecteru2/core/store/redis"
	"github.com/projecteru2/core/strategy"
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
	GetNodesByPod(ctx context.Context, podname string, labels map[string]string, all bool) ([]*types.Node, error)
	UpdateNodes(context.Context, ...*types.Node) error
	UpdateNodeResource(ctx context.Context, node *types.Node, resource *types.ResourceMeta, action string) error
	SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error
	GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error)
	NodeStatusStream(ctx context.Context) chan *types.NodeStatus

	// workload
	AddWorkload(context.Context, *types.Workload, *types.Processing) error
	UpdateWorkload(ctx context.Context, workload *types.Workload) error
	RemoveWorkload(ctx context.Context, workload *types.Workload) error
	GetWorkload(ctx context.Context, id string) (*types.Workload, error)
	GetWorkloads(ctx context.Context, ids []string) ([]*types.Workload, error)
	GetWorkloadStatus(ctx context.Context, id string) (*types.StatusMeta, error)
	SetWorkloadStatus(ctx context.Context, status *types.StatusMeta, ttl int64) error
	ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error)
	ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error)
	WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus

	// deploy status
	MakeDeployStatus(ctx context.Context, appname, entryname string, sis []strategy.Info) error

	// processing status
	CreateProcessing(ctx context.Context, process *types.Processing, count int) error
	DeleteProcessing(context.Context, *types.Processing) error

	// distributed lock
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)
}

// NewStore creates a store
func NewStore(config types.Config, t *testing.T) (store Store, err error) {
	switch config.Store {
	case types.Redis:
		store, err = redis.New(config, t)
	default:
		store, err = etcdv3.New(config, t)
	}
	return store, err
}
