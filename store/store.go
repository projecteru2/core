package store

import (
	"gitlab.ricebook.net/platform/core/lock"
	"gitlab.ricebook.net/platform/core/types"
)

type Store interface {
	// pod
	AddPod(name, desc string) (*types.Pod, error)
	GetPod(podname string) (*types.Pod, error)
	GetAllPods() ([]*types.Pod, error)

	// node
	AddNode(name, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error)
	DeleteNode(node *types.Node)
	GetNode(podname, nodename string) (*types.Node, error)
	GetNodesByPod(podname string) ([]*types.Node, error)
	GetAllNodes() ([]*types.Node, error)
	UpdateNode(*types.Node) error
	UpdateNodeCPU(podname, nodename string, cpu types.CPUMap, action string) error
	UpdateNodeMem(podname, nodename string, mem int64, action string) error

	// container
	AddContainer(id, podname, nodename, name string, cpu types.CPUMap, mem int64) (*types.Container, error)
	GetContainer(id string) (*types.Container, error)
	GetContainers(ids []string) ([]*types.Container, error)
	RemoveContainer(id string) error

	// distributed lock
	CreateLock(key string, ttl int) (lock.DistributedLock, error)

	// deploy status
	GetDeployStatus(opts *types.DeployOptions) ([]types.DeployStatus, error)
	UpdateDeployStatus(opts *types.DeployOptions, status []types.DeployStatus) error
	RemoveDeployStatus(opts *types.DeployOptions) error
}
