package store

import (
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
)

type Store interface {
	// pod
	AddPod(name, favor, desc string) (*types.Pod, error)
	GetPod(podname string) (*types.Pod, error)
	RemovePod(podname string) error
	GetAllPods() ([]*types.Pod, error)

	// node
	AddNode(name, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error)
	DeleteNode(node *types.Node)
	GetNode(podname, nodename string) (*types.Node, error)
	GetNodeByName(nodename string) (*types.Node, error)
	GetNodesByPod(podname string) ([]*types.Node, error)
	GetAllNodes() ([]*types.Node, error)
	UpdateNode(*types.Node) error
	UpdateNodeCPU(podname, nodename string, cpu types.CPUMap, action string) error
	UpdateNodeMem(podname, nodename string, mem int64, action string) error

	// container
	AddContainer(container *types.Container) error
	RemoveContainer(container *types.Container) error
	GetContainer(id string) (*types.Container, error)
	GetContainers(ids []string) ([]*types.Container, error)

	// distributed lock
	CreateLock(key string, ttl int) (lock.DistributedLock, error)

	// deploy status
	MakeDeployStatus(opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error)
}
