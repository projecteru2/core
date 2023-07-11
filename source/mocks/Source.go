// Code generated by mockery v2.30.1. DO NOT EDIT.

package mocks

import (
	context "context"

	mock "github.com/stretchr/testify/mock"
)

// Source is an autogenerated mock type for the Source type
type Source struct {
	mock.Mock
}

// Artifact provides a mock function with given fields: ctx, artifact, path
func (_m *Source) Artifact(ctx context.Context, artifact string, path string) error {
	ret := _m.Called(ctx, artifact, path)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) error); ok {
		r0 = rf(ctx, artifact, path)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Security provides a mock function with given fields: path
func (_m *Source) Security(path string) error {
	ret := _m.Called(path)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(path)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SourceCode provides a mock function with given fields: ctx, repository, path, revision, submodule
func (_m *Source) SourceCode(ctx context.Context, repository string, path string, revision string, submodule bool) error {
	ret := _m.Called(ctx, repository, path, revision, submodule)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, bool) error); ok {
		r0 = rf(ctx, repository, path, revision, submodule)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewSource creates a new instance of Source. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewSource(t interface {
	mock.TestingT
	Cleanup(func())
}) *Source {
	mock := &Source{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
