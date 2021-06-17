// Code generated by mockery v2.8.0. DO NOT EDIT.

package mocks

import (
	context "context"

	clientv3 "go.etcd.io/etcd/client/v3"

	lock "github.com/projecteru2/core/lock"

	mock "github.com/stretchr/testify/mock"

	mvccpb "go.etcd.io/etcd/api/v3/mvccpb"

	time "time"
)

// KV is an autogenerated mock type for the KV type
type KV struct {
	mock.Mock
}

// BatchCreate provides a mock function with given fields: ctx, data, opts
func (_m *KV) BatchCreate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, data)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.TxnResponse
	if rf, ok := ret.Get(0).(func(context.Context, map[string]string, ...clientv3.OpOption) *clientv3.TxnResponse); ok {
		r0 = rf(ctx, data, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.TxnResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, map[string]string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, data, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// BatchCreateAndDecr provides a mock function with given fields: ctx, data, decrKey
func (_m *KV) BatchCreateAndDecr(ctx context.Context, data map[string]string, decrKey string) error {
	ret := _m.Called(ctx, data, decrKey)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, map[string]string, string) error); ok {
		r0 = rf(ctx, data, decrKey)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// BatchDelete provides a mock function with given fields: ctx, keys, opts
func (_m *KV) BatchDelete(ctx context.Context, keys []string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, keys)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.TxnResponse
	if rf, ok := ret.Get(0).(func(context.Context, []string, ...clientv3.OpOption) *clientv3.TxnResponse); ok {
		r0 = rf(ctx, keys, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.TxnResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, keys, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// BatchUpdate provides a mock function with given fields: ctx, data, opts
func (_m *KV) BatchUpdate(ctx context.Context, data map[string]string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, data)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.TxnResponse
	if rf, ok := ret.Get(0).(func(context.Context, map[string]string, ...clientv3.OpOption) *clientv3.TxnResponse); ok {
		r0 = rf(ctx, data, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.TxnResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, map[string]string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, data, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// BindStatus provides a mock function with given fields: ctx, entityKey, statusKey, statusValue, ttl
func (_m *KV) BindStatus(ctx context.Context, entityKey string, statusKey string, statusValue string, ttl int64) error {
	ret := _m.Called(ctx, entityKey, statusKey, statusValue, ttl)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, int64) error); ok {
		r0 = rf(ctx, entityKey, statusKey, statusValue, ttl)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Create provides a mock function with given fields: ctx, key, val, opts
func (_m *KV) Create(ctx context.Context, key string, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key, val)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.TxnResponse
	if rf, ok := ret.Get(0).(func(context.Context, string, string, ...clientv3.OpOption) *clientv3.TxnResponse); ok {
		r0 = rf(ctx, key, val, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.TxnResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, key, val, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// CreateLock provides a mock function with given fields: key, ttl
func (_m *KV) CreateLock(key string, ttl time.Duration) (lock.DistributedLock, error) {
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

// Delete provides a mock function with given fields: ctx, key, opts
func (_m *KV) Delete(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.DeleteResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.DeleteResponse
	if rf, ok := ret.Get(0).(func(context.Context, string, ...clientv3.OpOption) *clientv3.DeleteResponse); ok {
		r0 = rf(ctx, key, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.DeleteResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, key, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Get provides a mock function with given fields: ctx, key, opts
func (_m *KV) Get(ctx context.Context, key string, opts ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.GetResponse
	if rf, ok := ret.Get(0).(func(context.Context, string, ...clientv3.OpOption) *clientv3.GetResponse); ok {
		r0 = rf(ctx, key, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.GetResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, key, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetMulti provides a mock function with given fields: ctx, keys, opts
func (_m *KV) GetMulti(ctx context.Context, keys []string, opts ...clientv3.OpOption) ([]*mvccpb.KeyValue, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, keys)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 []*mvccpb.KeyValue
	if rf, ok := ret.Get(0).(func(context.Context, []string, ...clientv3.OpOption) []*mvccpb.KeyValue); ok {
		r0 = rf(ctx, keys, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*mvccpb.KeyValue)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, []string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, keys, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetOne provides a mock function with given fields: ctx, key, opts
func (_m *KV) GetOne(ctx context.Context, key string, opts ...clientv3.OpOption) (*mvccpb.KeyValue, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *mvccpb.KeyValue
	if rf, ok := ret.Get(0).(func(context.Context, string, ...clientv3.OpOption) *mvccpb.KeyValue); ok {
		r0 = rf(ctx, key, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*mvccpb.KeyValue)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, key, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Grant provides a mock function with given fields: ctx, ttl
func (_m *KV) Grant(ctx context.Context, ttl int64) (*clientv3.LeaseGrantResponse, error) {
	ret := _m.Called(ctx, ttl)

	var r0 *clientv3.LeaseGrantResponse
	if rf, ok := ret.Get(0).(func(context.Context, int64) *clientv3.LeaseGrantResponse); ok {
		r0 = rf(ctx, ttl)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.LeaseGrantResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int64) error); ok {
		r1 = rf(ctx, ttl)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Put provides a mock function with given fields: ctx, key, val, opts
func (_m *KV) Put(ctx context.Context, key string, val string, opts ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key, val)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.PutResponse
	if rf, ok := ret.Get(0).(func(context.Context, string, string, ...clientv3.OpOption) *clientv3.PutResponse); ok {
		r0 = rf(ctx, key, val, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.PutResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, key, val, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// StartEphemeral provides a mock function with given fields: ctx, path, heartbeat
func (_m *KV) StartEphemeral(ctx context.Context, path string, heartbeat time.Duration) (<-chan struct{}, func(), error) {
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
func (_m *KV) TerminateEmbededStorage() {
	_m.Called()
}

// Update provides a mock function with given fields: ctx, key, val, opts
func (_m *KV) Update(ctx context.Context, key string, val string, opts ...clientv3.OpOption) (*clientv3.TxnResponse, error) {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key, val)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 *clientv3.TxnResponse
	if rf, ok := ret.Get(0).(func(context.Context, string, string, ...clientv3.OpOption) *clientv3.TxnResponse); ok {
		r0 = rf(ctx, key, val, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.TxnResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, string, string, ...clientv3.OpOption) error); ok {
		r1 = rf(ctx, key, val, opts...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Watch provides a mock function with given fields: ctx, key, opts
func (_m *KV) Watch(ctx context.Context, key string, opts ...clientv3.OpOption) clientv3.WatchChan {
	_va := make([]interface{}, len(opts))
	for _i := range opts {
		_va[_i] = opts[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, key)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 clientv3.WatchChan
	if rf, ok := ret.Get(0).(func(context.Context, string, ...clientv3.OpOption) clientv3.WatchChan); ok {
		r0 = rf(ctx, key, opts...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(clientv3.WatchChan)
		}
	}

	return r0
}
