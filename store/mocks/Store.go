// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocks

import (
	context "context"

	lock "github.com/projecteru2/core/lock"
	mock "github.com/stretchr/testify/mock"

	time "time"

	types "github.com/projecteru2/core/types"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// AddContainer provides a mock function with given fields: ctx, container
func (_m *Store) AddContainer(ctx context.Context, container *types.Container) error {
	ret := _m.Called(ctx, container)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Container) error); ok {
		r0 = rf(ctx, container)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddNode provides a mock function with given fields: ctx, name, endpoint, podname, ca, cert, key, cpu, share, memory, storage, labels, numa, numaMemory
func (_m *Store) AddNode(ctx context.Context, name string, endpoint string, podname string, ca string, cert string, key string, cpu int, share int, memory int64, storage int64, labels map[string]string, numa types.NUMA, numaMemory types.NUMAMemory, volumeMap types.VolumeMap) (*types.Node, error) {
	ret := _m.Called(ctx, name, endpoint, podname, ca, cert, key, cpu, share, memory, storage, labels, numa, numaMemory)

	var r0 *types.Node
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string, string, string, int, int, int64, int64, map[string]string, types.NUMA, types.NUMAMemory) *types.Node); ok {
		r0 = rf(ctx, name, endpoint, podname, ca, cert, key, cpu, share, memory, storage, labels, numa, numaMemory)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string, string, string, int, int, int64, int64, map[string]string, types.NUMA, types.NUMAMemory) error); ok {
		r1 = rf(ctx, name, endpoint, podname, ca, cert, key, cpu, share, memory, storage, labels, numa, numaMemory)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// AddPod provides a mock function with given fields: ctx, name, desc
func (_m *Store) AddPod(ctx context.Context, name string, desc string) (*types.Pod, error) {
	ret := _m.Called(ctx, name, desc)

	var r0 *types.Pod
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *types.Pod); ok {
		r0 = rf(ctx, name, desc)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Pod)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, name, desc)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ContainerStatusStream provides a mock function with given fields: ctx, appname, entrypoint, nodename, labels
func (_m *Store) ContainerStatusStream(ctx context.Context, appname string, entrypoint string, nodename string, labels map[string]string) chan *types.ContainerStatus {
	ret := _m.Called(ctx, appname, entrypoint, nodename, labels)

	var r0 chan *types.ContainerStatus
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, map[string]string) chan *types.ContainerStatus); ok {
		r0 = rf(ctx, appname, entrypoint, nodename, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *types.ContainerStatus)
		}
	}

	return r0
}

// CreateLock provides a mock function with given fields: key, ttl
func (_m *Store) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
	ret := _m.Called(key, ttl)

	var r0 lock.DistributedLock
	if rf, ok := ret.Get(0).(func(string, time.Duration) lock.DistributedLock); ok {
		r0 = rf(key, ttl)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(lock.DistributedLock)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, time.Duration) error); ok {
		r1 = rf(key, ttl)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteProcessing provides a mock function with given fields: ctx, opts, nodeInfo
func (_m *Store) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	ret := _m.Called(ctx, opts, nodeInfo)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, types.NodeInfo) error); ok {
		r0 = rf(ctx, opts, nodeInfo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetAllPods provides a mock function with given fields: ctx
func (_m *Store) GetAllPods(ctx context.Context) ([]*types.Pod, error) {
	ret := _m.Called(ctx)

	var r0 []*types.Pod
	if rf, ok := ret.Get(0).(func(context.Context) []*types.Pod); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Pod)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetContainer provides a mock function with given fields: ctx, ID
func (_m *Store) GetContainer(ctx context.Context, ID string) (*types.Container, error) {
	ret := _m.Called(ctx, ID)

	var r0 *types.Container
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.Container); ok {
		r0 = rf(ctx, ID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Container)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, ID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetContainerStatus provides a mock function with given fields: ctx, ID
func (_m *Store) GetContainerStatus(ctx context.Context, ID string) (*types.StatusMeta, error) {
	ret := _m.Called(ctx, ID)

	var r0 *types.StatusMeta
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.StatusMeta); ok {
		r0 = rf(ctx, ID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.StatusMeta)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, ID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetContainers provides a mock function with given fields: ctx, IDs
func (_m *Store) GetContainers(ctx context.Context, IDs []string) ([]*types.Container, error) {
	ret := _m.Called(ctx, IDs)

	var r0 []*types.Container
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*types.Container); ok {
		r0 = rf(ctx, IDs)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Container)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, IDs)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNode provides a mock function with given fields: ctx, nodename
func (_m *Store) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	ret := _m.Called(ctx, nodename)

	var r0 *types.Node
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.Node); ok {
		r0 = rf(ctx, nodename)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, nodename)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodes provides a mock function with given fields: ctx, nodenames
func (_m *Store) GetNodes(ctx context.Context, nodenames []string) ([]*types.Node, error) {
	ret := _m.Called(ctx, nodenames)

	var r0 []*types.Node
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*types.Node); ok {
		r0 = rf(ctx, nodenames)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, nodenames)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodesByPod provides a mock function with given fields: ctx, podname, labels, all
func (_m *Store) GetNodesByPod(ctx context.Context, podname string, labels map[string]string, all bool) ([]*types.Node, error) {
	ret := _m.Called(ctx, podname, labels, all)

	var r0 []*types.Node
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string, bool) []*types.Node); ok {
		r0 = rf(ctx, podname, labels, all)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string, bool) error); ok {
		r1 = rf(ctx, podname, labels, all)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetPod provides a mock function with given fields: ctx, podname
func (_m *Store) GetPod(ctx context.Context, podname string) (*types.Pod, error) {
	ret := _m.Called(ctx, podname)

	var r0 *types.Pod
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.Pod); ok {
		r0 = rf(ctx, podname)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Pod)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, podname)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListContainers provides a mock function with given fields: ctx, appname, entrypoint, nodename, limit, labels
func (_m *Store) ListContainers(ctx context.Context, appname string, entrypoint string, nodename string, limit int64, labels map[string]string) ([]*types.Container, error) {
	ret := _m.Called(ctx, appname, entrypoint, nodename, limit, labels)

	var r0 []*types.Container
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, int64, map[string]string) []*types.Container); ok {
		r0 = rf(ctx, appname, entrypoint, nodename, limit, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Container)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, int64, map[string]string) error); ok {
		r1 = rf(ctx, appname, entrypoint, nodename, limit, labels)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListNodeContainers provides a mock function with given fields: ctx, nodename, labels
func (_m *Store) ListNodeContainers(ctx context.Context, nodename string, labels map[string]string) ([]*types.Container, error) {
	ret := _m.Called(ctx, nodename, labels)

	var r0 []*types.Container
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) []*types.Container); ok {
		r0 = rf(ctx, nodename, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Container)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, map[string]string) error); ok {
		r1 = rf(ctx, nodename, labels)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MakeDeployStatus provides a mock function with given fields: ctx, opts, nodesInfo
func (_m *Store) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) ([]types.NodeInfo, error) {
	ret := _m.Called(ctx, opts, nodesInfo)

	var r0 []types.NodeInfo
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, []types.NodeInfo) []types.NodeInfo); ok {
		r0 = rf(ctx, opts, nodesInfo)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]types.NodeInfo)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *types.DeployOptions, []types.NodeInfo) error); ok {
		r1 = rf(ctx, opts, nodesInfo)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveContainer provides a mock function with given fields: ctx, container
func (_m *Store) RemoveContainer(ctx context.Context, container *types.Container) error {
	ret := _m.Called(ctx, container)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Container) error); ok {
		r0 = rf(ctx, container)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemoveNode provides a mock function with given fields: ctx, node
func (_m *Store) RemoveNode(ctx context.Context, node *types.Node) error {
	ret := _m.Called(ctx, node)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Node) error); ok {
		r0 = rf(ctx, node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RemovePod provides a mock function with given fields: ctx, podname
func (_m *Store) RemovePod(ctx context.Context, podname string) error {
	ret := _m.Called(ctx, podname)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, podname)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveProcessing provides a mock function with given fields: ctx, opts, nodeInfo
func (_m *Store) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodeInfo types.NodeInfo) error {
	ret := _m.Called(ctx, opts, nodeInfo)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, types.NodeInfo) error); ok {
		r0 = rf(ctx, opts, nodeInfo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetContainerStatus provides a mock function with given fields: ctx, container, ttl
func (_m *Store) SetContainerStatus(ctx context.Context, container *types.Container, ttl int64) error {
	ret := _m.Called(ctx, container, ttl)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Container, int64) error); ok {
		r0 = rf(ctx, container, ttl)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// TerminateEmbededStorage provides a mock function with given fields:
func (_m *Store) TerminateEmbededStorage() {
	_m.Called()
}

// UpdateContainer provides a mock function with given fields: ctx, container
func (_m *Store) UpdateContainer(ctx context.Context, container *types.Container) error {
	ret := _m.Called(ctx, container)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Container) error); ok {
		r0 = rf(ctx, container)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateNode provides a mock function with given fields: ctx, node
func (_m *Store) UpdateNode(ctx context.Context, node *types.Node) error {
	ret := _m.Called(ctx, node)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Node) error); ok {
		r0 = rf(ctx, node)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateNodeResource provides a mock function with given fields: ctx, node, cpu, quota, memory, storage, action
func (_m *Store) UpdateNodeResource(ctx context.Context, node *types.Node, cpu types.CPUMap, quota float64, memory int64, storage int64, volume types.VolumeMap, action string) error {
	ret := _m.Called(ctx, node, cpu, quota, memory, storage, action)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Node, types.CPUMap, float64, int64, int64, string) error); ok {
		r0 = rf(ctx, node, cpu, quota, memory, storage, action)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateProcessing provides a mock function with given fields: ctx, opts, nodename, count
func (_m *Store) UpdateProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	ret := _m.Called(ctx, opts, nodename, count)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, string, int) error); ok {
		r0 = rf(ctx, opts, nodename, count)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
