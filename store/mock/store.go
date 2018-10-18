package mockstore

import (
	"context"
	"fmt"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

// MockLock for mock lock
type MockLock struct {
	mock.Mock
}

// Lock mock lock
func (m *MockLock) Lock(ctx context.Context) error {
	args := m.Called()
	return args.Error(0)
}

// Unlock mock unlock
func (m *MockLock) Unlock(ctx context.Context) error {
	args := m.Called()
	return args.Error(0)
}

// MockStore mock store
type MockStore struct {
	mock.Mock
}

// WatchDeployStatus fake watch deploy status
func (m *MockStore) WatchDeployStatus(appname, entrypoint, nodename string) etcdclient.Watcher {
	args := m.Called(appname, entrypoint, nodename)
	return args.Get(0).(etcdclient.Watcher)
}

// GetPod fake get pod
func (m *MockStore) GetPod(ctx context.Context, name string) (*types.Pod, error) {
	args := m.Called(name)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
}

// AddPod fake add pod
func (m *MockStore) AddPod(ctx context.Context, name, favor, desc string) (*types.Pod, error) {
	args := m.Called(name, favor, desc)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
}

// RemovePod fake remove pod
func (m *MockStore) RemovePod(ctx context.Context, podname string) error {
	nodes, err := m.GetNodesByPod(ctx, podname)
	if err != nil {
		return err
	}
	if l := len(nodes); l != 0 {
		return types.NewDetailedErr(types.ErrPodHasNodes,
			fmt.Sprintf("pod %s still has %d nodes, delete them first", podname, l))
	}
	args := m.Called(ctx, podname)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	return args.Error(0)
}

// GetAllPods fake get all pods
func (m *MockStore) GetAllPods(ctx context.Context) ([]*types.Pod, error) {
	args := m.Called()
	return args.Get(0).([]*types.Pod), args.Error(1)
}

// GetNode fake get node
func (m *MockStore) GetNode(ctx context.Context, pod, node string) (*types.Node, error) {
	args := m.Called(pod, node)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

// AddNode fake add node
func (m *MockStore) AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu, share int, memory int64, labels map[string]string) (*types.Node, error) {
	args := m.Called(name, endpoint, podname, ca, cert, key, cpu, share, memory, labels)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

// ListContainers fake list containers
func (m *MockStore) ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error) {
	args := m.Called(appname, entrypoint, nodename)
	if args.Get(0) != nil {
		return args.Get(0).([]*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

// ListNodeContainers fake list node containers
func (m *MockStore) ListNodeContainers(ctx context.Context, nodename string) ([]*types.Container, error) {
	args := m.Called(nodename)
	if args.Get(0) != nil {
		return args.Get(0).([]*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

// CleanContainerData fake clean container data
func (m *MockStore) CleanContainerData(ctx context.Context, ID, appname, entrypoint, nodename string) error {
	args := m.Called(ID, appname, entrypoint, nodename)
	return args.Error(0)
}

// DeleteNode fake delete node
func (m *MockStore) DeleteNode(ctx context.Context, node *types.Node) {
	m.Called(node)
}

// GetNodeByName fake get node by name
func (m *MockStore) GetNodeByName(ctx context.Context, node string) (*types.Node, error) {
	args := m.Called(node)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

// GetAllNodes fake get all nodes
func (m *MockStore) GetAllNodes(ctx context.Context) ([]*types.Node, error) {
	args := m.Called()
	return args.Get(0).([]*types.Node), args.Error(1)
}

// GetNodesByPod fake get nodes by pod
func (m *MockStore) GetNodesByPod(ctx context.Context, name string) ([]*types.Node, error) {
	args := m.Called(name)
	return args.Get(0).([]*types.Node), args.Error(1)
}

// UpdateNode fake update node
func (m *MockStore) UpdateNode(ctx context.Context, node *types.Node) error {
	args := m.Called(node)
	return args.Error(0)
}

// UpdateNodeResource fake update node resource
func (m *MockStore) UpdateNodeResource(ctx context.Context, podname, nodename string, cpu types.CPUMap, mem int64, action string) error {
	args := m.Called(podname, nodename, cpu, mem, action)
	return args.Error(0)
}

// GetContainer fake get container
func (m *MockStore) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	if len(ID) != 64 {
		return nil, types.ErrBadContainerID
	}
	args := m.Called(ID)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

// GetContainers fake get containers
func (m *MockStore) GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error) {
	args := m.Called(IDs)
	return args.Get(0).([]*types.Container), args.Error(1)
}

// AddContainer fake add container
func (m *MockStore) AddContainer(ctx context.Context, container *types.Container) error {
	// since we add the container ourselves, no one cares about the return
	return nil
}

// RemoveContainer fake remove container
func (m *MockStore) RemoveContainer(ctx context.Context, container *types.Container) error {
	args := m.Called(container)
	return args.Error(0)
}

// CreateLock fake create lock
func (m *MockStore) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	args := m.Called(key, ttl)
	if args.Get(0) != nil {
		return args.Get(0).(*MockLock), args.Error(1)
	}
	return nil, args.Error(1)
}

// MakeDeployStatus fake make deploy status
func (m *MockStore) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	args := m.Called(opts, nodesInfo)
	if args.Get(0) != nil {
		return args.Get(0).([]types.NodeInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

// ContainerDeployed fake container deployed
func (m *MockStore) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error {
	args := m.Called(ID, appname, entrypoint, nodename, data)
	return args.Error(0)
}

// SaveProcessing save processing status in etcd
func (m *MockStore) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	args := m.Called(ctx, opts, nodeInfo)
	return args.Error(0)
}

// UpdateProcessing update processing status in etcd
func (m *MockStore) UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	args := m.Called(ctx, opts, nodename, count)
	return args.Error(0)
}

// DeleteProcessing delete processing status in etcd
func (m *MockStore) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	args := m.Called(ctx, opts, nodeInfo)
	return args.Error(0)
}
