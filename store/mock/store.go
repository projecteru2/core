package mockstore

import (
	"fmt"

	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

type MockLock struct {
	mock.Mock
}

func (m *MockLock) Lock() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockLock) Unlock() error {
	args := m.Called()
	return args.Error(0)
}

type MockStore struct {
	mock.Mock
}

func (m *MockStore) GetPod(name string) (*types.Pod, error) {
	args := m.Called(name)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) AddPod(name, favor, desc string) (*types.Pod, error) {
	args := m.Called(name, favor, desc)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) RemovePod(podname string) error {
	nodes, err := m.GetNodesByPod(podname)
	if err != nil {
		return err
	}
	if len(nodes) != 0 {
		return fmt.Errorf("[RemovePod] pod %s still has nodes, delete the nodes first", podname)
	}
	args := m.Called(podname)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	return args.Error(0)
}

func (m *MockStore) GetAllPods() ([]*types.Pod, error) {
	args := m.Called()
	return args.Get(0).([]*types.Pod), args.Error(1)
}

func (m *MockStore) GetNode(pod, node string) (*types.Node, error) {
	args := m.Called(pod, node)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) AddNode(name, endpoint, podname, cafile, certfile, keyfile string, public bool) (*types.Node, error) {
	args := m.Called(name, endpoint, podname, cafile, certfile, keyfile, public)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Node), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) DeleteNode(node *types.Node) {
	m.Called(node)
}

func (m *MockStore) GetAllNodes() ([]*types.Node, error) {
	args := m.Called()
	return args.Get(0).([]*types.Node), args.Error(1)
}

func (m *MockStore) GetNodesByPod(name string) ([]*types.Node, error) {
	args := m.Called(name)
	return args.Get(0).([]*types.Node), args.Error(1)
}

func (m *MockStore) UpdateNode(node *types.Node) error {
	args := m.Called(node)
	return args.Error(0)
}

func (m *MockStore) UpdateNodeCPU(podname, nodename string, cpu types.CPUMap, action string) error {
	args := m.Called(podname, nodename, cpu, action)
	return args.Error(0)
}

func (m *MockStore) UpdateNodeMem(podname, nodename string, memory int64, action string) error {
	args := m.Called(podname, nodename, memory, action)
	return args.Error(0)
}

func (m *MockStore) GetContainer(id string) (*types.Container, error) {
	if len(id) != 64 {
		return nil, fmt.Errorf("Container ID must be length of 64")
	}
	args := m.Called(id)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) GetContainers(ids []string) ([]*types.Container, error) {
	args := m.Called(ids)
	return args.Get(0).([]*types.Container), args.Error(1)
}

func (m *MockStore) AddContainer(id, podname, nodename, name string, cpu types.CPUMap, memory int64) (*types.Container, error) {
	// since we add the container ourselves, no one cares about the return
	return nil, nil
}

func (m *MockStore) RemoveContainer(id string, container *types.Container) error {
	args := m.Called(id, container)
	return args.Error(0)
}

func (m *MockStore) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	args := m.Called(key, ttl)
	if args.Get(0) != nil {
		return args.Get(0).(*MockLock), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) MakeDeployStatus(opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	args := m.Called(opts, nodesInfo)
	if args.Get(0) != nil {
		return args.Get(0).([]types.NodeInfo), args.Error(1)
	}
	return nil, args.Error(1)
}
