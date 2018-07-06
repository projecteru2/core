package store

import (
	"context"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
)

//Store store eru data
type Store interface {
	// pod
	AddPod(ctx context.Context, name, favor, desc string) (*types.Pod, error)
	GetPod(ctx context.Context, podname string) (*types.Pod, error)
	RemovePod(ctx context.Context, podname string) error
	GetAllPods(ctx context.Context) ([]*types.Pod, error)

	// node
	AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu int, share, memory int64, labels map[string]string) (*types.Node, error)
	DeleteNode(ctx context.Context, node *types.Node)
	GetNode(ctx context.Context, podname, nodename string) (*types.Node, error)
	GetNodeByName(ctx context.Context, nodename string) (*types.Node, error)
	GetNodesByPod(ctx context.Context, podname string) ([]*types.Node, error)
	GetAllNodes(ctx context.Context) ([]*types.Node, error)
	UpdateNode(ctx context.Context, node *types.Node) error
	UpdateNodeResource(ctx context.Context, podname, nodename string, cpu types.CPUMap, mem int64, action string) error

	// container
	AddContainer(ctx context.Context, container *types.Container) error
	RemoveContainer(ctx context.Context, container *types.Container) error
	CleanContainerData(ctx context.Context, ID, appname, entrypoint, nodename string) error
	GetContainer(ctx context.Context, ID string) (*types.Container, error)
	GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error)
	ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error
	ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error)

	// deploy status
	MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error)

	// distributed lock
	CreateLock(key string, ttl int) (lock.DistributedLock, error)
	// watch deploy status
	WatchDeployStatus(appname, entrypoint, nodename string) etcdclient.Watcher
}
