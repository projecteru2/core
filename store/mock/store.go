package mockstore

import (
	"github.com/stretchr/testify/mock"
	"gitlab.ricebook.net/platform/core/types"
)

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

func (m *MockStore) AddNode(name, endpoint, podname string, public bool) (*types.Node, error) {
	args := m.Called(name, endpoint, podname, public)
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

func (m *MockStore) AddContainer(id, podname, nodename string) (*types.Container, error) {
	args := m.Called(id, podname, nodename)
	if args.Get(0) != nil {
		return args.Get(0).(*types.Container), args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockStore) RemoveContainer(id string) error {
	args := m.Called(id)
	return args.Error(0)
}
