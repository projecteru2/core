// Code generated by mockery v2.10.0. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	clientv3 "go.etcd.io/etcd/client/v3"
)

// Txn is an autogenerated mock type for the Txn type
type Txn struct {
	mock.Mock
}

// Commit provides a mock function with given fields:
func (_m *Txn) Commit() (*clientv3.TxnResponse, error) {
	ret := _m.Called()

	var r0 *clientv3.TxnResponse
	if rf, ok := ret.Get(0).(func() *clientv3.TxnResponse); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*clientv3.TxnResponse)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Else provides a mock function with given fields: ops
func (_m *Txn) Else(ops ...clientv3.Op) clientv3.Txn {
	_va := make([]interface{}, len(ops))
	for _i := range ops {
		_va[_i] = ops[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 clientv3.Txn
	if rf, ok := ret.Get(0).(func(...clientv3.Op) clientv3.Txn); ok {
		r0 = rf(ops...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(clientv3.Txn)
		}
	}

	return r0
}

// If provides a mock function with given fields: cs
func (_m *Txn) If(cs ...clientv3.Cmp) clientv3.Txn {
	_va := make([]interface{}, len(cs))
	for _i := range cs {
		_va[_i] = cs[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 clientv3.Txn
	if rf, ok := ret.Get(0).(func(...clientv3.Cmp) clientv3.Txn); ok {
		r0 = rf(cs...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(clientv3.Txn)
		}
	}

	return r0
}

// Then provides a mock function with given fields: ops
func (_m *Txn) Then(ops ...clientv3.Op) clientv3.Txn {
	_va := make([]interface{}, len(ops))
	for _i := range ops {
		_va[_i] = ops[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	var r0 clientv3.Txn
	if rf, ok := ret.Get(0).(func(...clientv3.Op) clientv3.Txn); ok {
		r0 = rf(ops...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(clientv3.Txn)
		}
	}

	return r0
}
