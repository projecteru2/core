package mockstore

import (
	"github.com/stretchr/testify/mock"
	"gitlab.ricebook.net/platform/core/lock"
	"gitlab.ricebook.net/platform/core/types"
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

func (m *MockStore) AddPod(name, desc string) (*types.Pod, error) {
	args := m.Called(name, desc)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Pod), args.Error(1)
	}
	return nil, args.Error(1)
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
	args := m.Called(id, podname, nodename, name, cpu, memory)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) RemoveContainer(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockStore) CreateLock(key string, ttl int) (lock.DistributedLock, error) {
	args := m.Called(key, ttl)
	if args.Get(0) != nil {
		return args.Get(0).(*MockLock), args.Error(1)
	}
	return nil, args.Error(1)
}
