// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	context "context"

	lock "github.com/projecteru2/core/lock"
	mock "github.com/stretchr/testify/mock"

	strategy "github.com/projecteru2/core/strategy"

	time "time"

	types "github.com/projecteru2/core/types"
)

// Store is an autogenerated mock type for the Store type
type Store struct {
	mock.Mock
}

// AddNode provides a mock function with given fields: _a0, _a1
func (_m *Store) AddNode(_a0 context.Context, _a1 *types.AddNodeOptions) (*types.Node, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *types.Node
	if rf, ok := ret.Get(0).(func(context.Context, *types.AddNodeOptions) *types.Node); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Node)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, *types.AddNodeOptions) error); ok {
		r1 = rf(_a0, _a1)
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

// AddWorkload provides a mock function with given fields: ctx, workload
func (_m *Store) AddWorkload(ctx context.Context, workload *types.Workload) error {
	ret := _m.Called(ctx, workload)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Workload) error); ok {
		r0 = rf(ctx, workload)
	} else {
		r0 = ret.Error(0)
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

// DeleteProcessing provides a mock function with given fields: ctx, opts, nodename
func (_m *Store) DeleteProcessing(ctx context.Context, opts *types.DeployOptions, nodename string) error {
	ret := _m.Called(ctx, opts, nodename)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, string) error); ok {
		r0 = rf(ctx, opts, nodename)
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

// GetNodeStatus provides a mock function with given fields: ctx, nodename
func (_m *Store) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	ret := _m.Called(ctx, nodename)

	var r0 *types.NodeStatus
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.NodeStatus); ok {
		r0 = rf(ctx, nodename)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.NodeStatus)
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

// GetWorkload provides a mock function with given fields: ctx, id
func (_m *Store) GetWorkload(ctx context.Context, id string) (*types.Workload, error) {
	ret := _m.Called(ctx, id)

	var r0 *types.Workload
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.Workload); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Workload)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkloadStatus provides a mock function with given fields: ctx, id
func (_m *Store) GetWorkloadStatus(ctx context.Context, id string) (*types.StatusMeta, error) {
	ret := _m.Called(ctx, id)

	var r0 *types.StatusMeta
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.StatusMeta); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.StatusMeta)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetWorkloads provides a mock function with given fields: ctx, ids
func (_m *Store) GetWorkloads(ctx context.Context, ids []string) ([]*types.Workload, error) {
	ret := _m.Called(ctx, ids)

	var r0 []*types.Workload
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*types.Workload); ok {
		r0 = rf(ctx, ids)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Workload)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, ids)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListNodeWorkloads provides a mock function with given fields: ctx, nodename, labels
func (_m *Store) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error) {
	ret := _m.Called(ctx, nodename, labels)

	var r0 []*types.Workload
	if rf, ok := ret.Get(0).(func(context.Context, string, map[string]string) []*types.Workload); ok {
		r0 = rf(ctx, nodename, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Workload)
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

// ListWorkloads provides a mock function with given fields: ctx, appname, entrypoint, nodename, limit, labels
func (_m *Store) ListWorkloads(ctx context.Context, appname string, entrypoint string, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error) {
	ret := _m.Called(ctx, appname, entrypoint, nodename, limit, labels)

	var r0 []*types.Workload
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, int64, map[string]string) []*types.Workload); ok {
		r0 = rf(ctx, appname, entrypoint, nodename, limit, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Workload)
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

// MakeDeployStatus provides a mock function with given fields: ctx, opts, strategyInfo
func (_m *Store) MakeDeployStatus(ctx context.Context, opts *types.DeployOptions, strategyInfo []strategy.Info) error {
	ret := _m.Called(ctx, opts, strategyInfo)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, []strategy.Info) error); ok {
		r0 = rf(ctx, opts, strategyInfo)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NodeStatusStream provides a mock function with given fields: ctx
func (_m *Store) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	ret := _m.Called(ctx)

	var r0 chan *types.NodeStatus
	if rf, ok := ret.Get(0).(func(context.Context) chan *types.NodeStatus); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *types.NodeStatus)
		}
	}

	return r0
}

// RegisterService provides a mock function with given fields: _a0, _a1, _a2
func (_m *Store) RegisterService(_a0 context.Context, _a1 string, _a2 time.Duration) (<-chan struct{}, func(), error) {
	ret := _m.Called(_a0, _a1, _a2)

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Duration) <-chan struct{}); ok {
		r0 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	var r1 func()
	if rf, ok := ret.Get(1).(func(context.Context, string, time.Duration) func()); ok {
		r1 = rf(_a0, _a1, _a2)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(func())
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, string, time.Duration) error); ok {
		r2 = rf(_a0, _a1, _a2)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
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

// RemoveWorkload provides a mock function with given fields: ctx, workload
func (_m *Store) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	ret := _m.Called(ctx, workload)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Workload) error); ok {
		r0 = rf(ctx, workload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SaveProcessing provides a mock function with given fields: ctx, opts, nodename, count
func (_m *Store) SaveProcessing(ctx context.Context, opts *types.DeployOptions, nodename string, count int) error {
	ret := _m.Called(ctx, opts, nodename, count)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.DeployOptions, string, int) error); ok {
		r0 = rf(ctx, opts, nodename, count)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ServiceStatusStream provides a mock function with given fields: _a0
func (_m *Store) ServiceStatusStream(_a0 context.Context) (chan []string, error) {
	ret := _m.Called(_a0)

	var r0 chan []string
	if rf, ok := ret.Get(0).(func(context.Context) chan []string); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan []string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(_a0)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SetNodeStatus provides a mock function with given fields: ctx, node, ttl
func (_m *Store) SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error {
	ret := _m.Called(ctx, node, ttl)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Node, int64) error); ok {
		r0 = rf(ctx, node, ttl)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetWorkloadStatus provides a mock function with given fields: ctx, workload, ttl
func (_m *Store) SetWorkloadStatus(ctx context.Context, workload *types.Workload, ttl int64) error {
	ret := _m.Called(ctx, workload, ttl)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Workload, int64) error); ok {
		r0 = rf(ctx, workload, ttl)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// StartEphemeral provides a mock function with given fields: ctx, path, heartbeat
func (_m *Store) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
	ret := _m.Called(ctx, path, heartbeat)

	var r0 <-chan struct{}
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Duration) <-chan struct{}); ok {
		r0 = rf(ctx, path, heartbeat)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(<-chan struct{})
		}
	}

	var r1 func()
	if rf, ok := ret.Get(1).(func(context.Context, string, time.Duration) func()); ok {
		r1 = rf(ctx, path, heartbeat)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(func())
		}
	}

	var r2 error
	if rf, ok := ret.Get(2).(func(context.Context, string, time.Duration) error); ok {
		r2 = rf(ctx, path, heartbeat)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// TerminateEmbededStorage provides a mock function with given fields:
func (_m *Store) TerminateEmbededStorage() {
	_m.Called()
}

// UpdateNodeResource provides a mock function with given fields: ctx, node, resource, action
func (_m *Store) UpdateNodeResource(ctx context.Context, node *types.Node, resource *types.ResourceMeta, action string) error {
	ret := _m.Called(ctx, node, resource, action)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Node, *types.ResourceMeta, string) error); ok {
		r0 = rf(ctx, node, resource, action)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateNodes provides a mock function with given fields: _a0, _a1
func (_m *Store) UpdateNodes(_a0 context.Context, _a1 ...*types.Node) error {
	_va := make([]interface{}, len(_a1))
	for _i := range _a1 {
		_va[_i] = _a1[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _a0)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, ...*types.Node) error); ok {
		r0 = rf(_a0, _a1...)
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

// UpdateWorkload provides a mock function with given fields: ctx, workload
func (_m *Store) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	ret := _m.Called(ctx, workload)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.Workload) error); ok {
		r0 = rf(ctx, workload)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// WorkloadStatusStream provides a mock function with given fields: ctx, appname, entrypoint, nodename, labels
func (_m *Store) WorkloadStatusStream(ctx context.Context, appname string, entrypoint string, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	ret := _m.Called(ctx, appname, entrypoint, nodename, labels)

	var r0 chan *types.WorkloadStatus
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, map[string]string) chan *types.WorkloadStatus); ok {
		r0 = rf(ctx, appname, entrypoint, nodename, labels)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(chan *types.WorkloadStatus)
		}
	}

	return r0
}
