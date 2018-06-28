package mockstore

import (
	"context"
	"fmt"

	etcdclient "github.com/coreos/etcd/client"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

type MockLock struct {
	mock.Mock
}

func (m *MockLock) Lock(ctx context.Context) error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockLock) Unlock(ctx context.Context) error {
	args := m.Called()
	return args.Error(0)
}

type MockStore struct {
	mock.Mock
}

func (m *MockStore) WatchDeployStatus(appname, entrypoint, nodename string) etcdclient.Watcher {
	args := m.Called(appname, entrypoint, nodename)
	return args.Get(0).(etcdclient.Watcher)
}

func (m *MockStore) GetPod(ctx context.Context, name string) (*types.Pod, error) {
	args := m.Called(name)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) AddPod(ctx context.Context, name, favor, desc string) (*types.Pod, error) {
	args := m.Called(name, favor, desc)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) RemovePod(ctx context.Context, podname string) error {
	nodes, err := m.GetNodesByPod(ctx, podname)
	if err != nil {
		return err
	}
	if len(nodes) != 0 {
		return fmt.Errorf("Pod %s still has nodes, delete the nodes first", podname)
	}
	args := m.Called(ctx, podname)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	return args.Error(0)
}

func (m *MockStore) GetAllPods(ctx context.Context) ([]*types.Pod, error) {
	args := m.Called()
	return args.Get(0).([]*types.Pod), args.Error(1)
}

func (m *MockStore) GetNode(ctx context.Context, pod, node string) (*types.Node, error) {
	args := m.Called(pod, node)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu int, share, memory int64, labels map[string]string) (*types.Node, error) {
	args := m.Called(name, endpoint, podname, ca, cert, key, cpu, share, memory, labels)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) ListContainers(ctx context.Context, appname, entrypoint, nodename string) ([]*types.Container, error) {
	args := m.Called(appname, entrypoint, nodename)
	if args.Get(0) != nil {
		return args.Get(0).([]*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) CleanContainerData(ctx context.Context, ID, appname, entrypoint, nodename string) error {
	args := m.Called(ID, appname, entrypoint, nodename)
	return args.Error(0)
}

func (m *MockStore) DeleteNode(ctx context.Context, node *types.Node) {
	m.Called(node)
}

func (m *MockStore) GetNodeByName(ctx context.Context, node string) (*types.Node, error) {
	args := m.Called(node)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) GetAllNodes(ctx context.Context) ([]*types.Node, error) {
	args := m.Called()
	return args.Get(0).([]*types.Node), args.Error(1)
}

func (m *MockStore) GetNodesByPod(ctx context.Context, name string) ([]*types.Node, error) {
	args := m.Called(name)
	return args.Get(0).([]*types.Node), args.Error(1)
}

func (m *MockStore) UpdateNode(ctx context.Context, node *types.Node) error {
	args := m.Called(node)
	return args.Error(0)
}

func (m *MockStore) UpdateNodeCPU(ctx context.Context, podname, nodename string, cpu types.CPUMap, action string) error {
	args := m.Called(podname, nodename, cpu, action)
	return args.Error(0)
}

func (m *MockStore) UpdateNodeMem(ctx context.Context, podname, nodename string, memory int64, action string) error {
	args := m.Called(podname, nodename, memory, action)
	return args.Error(0)
}

func (m *MockStore) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	if len(ID) != 64 {
		return nil, fmt.Errorf("Container ID must be length of 64")
	}
	args := m.Called(ID)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error) {
	args := m.Called(IDs)
	return args.Get(0).([]*types.Container), args.Error(1)
}

func (m *MockStore) AddContainer(ctx context.Context, container *types.Container) error {
	// since we add the container ourselves, no one cares about the return
	return nil
}

func (m *MockStore) RemoveContainer(ctx context.Context, container *types.Container) error {
	args := m.Called(container)
	return args.Error(0)
}

func (m *MockStore) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	args := m.Called(key, ttl)
	if args.Get(0) != nil {
		return args.Get(0).(*MockLock), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	args := m.Called(opts, nodesInfo)
	if args.Get(0) != nil {
		return args.Get(0).([]types.NodeInfo), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) ContainerDeployed(ctx context.Context, ID, appname, entrypoint, nodename, data string) error {
	args := m.Called(ID, appname, entrypoint, nodename, data)
	return args.Error(0)
}
