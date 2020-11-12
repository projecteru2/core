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
	RegisterService(context.Context, string, time.Duration) error
	UnregisterService(context.Context, string) error

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

	// container
	AddContainer(ctx context.Context, container *types.Container) error
	UpdateContainer(ctx context.Context, container *types.Container) error
	RemoveContainer(ctx context.Context, container *types.Container) error
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	GetContainerStatus(ctx context.Context, ID string) (*types.StatusMeta, error)
	SetContainerStatus(ctx context.Context, container *types.Container, ttl int64) error
	ListContainers(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Container, error)
	ListNodeContainers(ctx context.Context, nodename string, labels map[string]string) ([]*types.Container, error)
	ContainerStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.ContainerStatus

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
