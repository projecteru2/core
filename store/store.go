package store

import (
	"context"
	"time"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
)

const (
	// ActionIncr for incr resource
	ActionIncr = "+"
	// ActionDecr for decr resource
	ActionDecr = "-"
)

//Store store eru data
type Store interface {
	// pod
	AddPod(ctx context.Context, name, desc string) (*types.Pod, error)
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	RemovePod(ctx context.Context, podname string) error
	GetAllPods(ctx context.Context) ([]*types.Pod, error)

	// node
	AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string,
		cpu, share int, memory, storage int64, labels map[string]string,
		numa types.NUMA, numaMemory types.NUMAMemory) (*types.Node, error)
	DeleteNode(ctx context.Context, node *types.Node) error
	GetNode(ctx context.Context, podname, nodename string) (*types.Node, error)
	GetNodeByName(ctx context.Context, nodename string) (*types.Node, error)
	GetNodesByPod(ctx context.Context, podname string) ([]*types.Node, error)
	UpdateNode(ctx context.Context, node *types.Node) error
	UpdateNodeResource(ctx context.Context, node *types.Node, cpu types.CPUMap, quota float64, memory, storage int64, action string) error

	// container
	AddContainer(ctx context.Context, container *types.Container) error
	UpdateContainer(ctx context.Context, container *types.Container) error
	RemoveContainer(ctx context.Context, container *types.Container) error
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	ListContainers(ctx context.Context, appname, entrypoint, nodename string, limit int64) ([]*types.Container, error)
	ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error)
	GetContainerStatus(ctx context.Context, ID string) (types.StatusMeta, error)
	SetContainerStatus(ctx context.Context, container *types.Container, data []byte, ttl int64) (types.StatusMeta, error)
	ContainerStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.ContainerStatus

	// deploy status
	MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error)

	// processing status
	SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error
	UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error
	DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error

	// distributed lock
	CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error)

	// embeded storage
	TerminateEmbededStorage()
}
