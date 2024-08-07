// Code generated by mockery v2.42.0. DO NOT EDIT.

package mocks

import (
	context "context"

	io "io"

	mock "github.com/stretchr/testify/mock"

	resourcetypes "github.com/projecteru2/core/resource/types"

	source "github.com/projecteru2/core/source"

	time "time"

	types "github.com/projecteru2/core/engine/types"
)

// API is an autogenerated mock type for the API type
type API struct {
	mock.Mock
}

// BuildContent provides a mock function with given fields: ctx, scm, opts
func (_m *API) BuildContent(ctx context.Context, scm source.Source, opts *types.BuildContentOptions) (string, io.Reader, error) {
	ret := _m.Called(ctx, scm, opts)

	if len(ret) == 0 {
		panic("no return value specified for BuildContent")
	}

	var r0 string
	var r1 io.Reader
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, source.Source, *types.BuildContentOptions) (string, io.Reader, error)); ok {
		return rf(ctx, scm, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, source.Source, *types.BuildContentOptions) string); ok {
		r0 = rf(ctx, scm, opts)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, source.Source, *types.BuildContentOptions) io.Reader); ok {
		r1 = rf(ctx, scm, opts)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.Reader)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, source.Source, *types.BuildContentOptions) error); ok {
		r2 = rf(ctx, scm, opts)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// BuildRefs provides a mock function with given fields: ctx, opts
func (_m *API) BuildRefs(ctx context.Context, opts *types.BuildRefOptions) []string {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for BuildRefs")
	}

	var r0 []string
	if rf, ok := ret.Get(0).(func(context.Context, *types.BuildRefOptions) []string); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}

// CloseConn provides a mock function with given fields:
func (_m *API) CloseConn() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for CloseConn")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// ExecExitCode provides a mock function with given fields: ctx, ID, execID
func (_m *API) ExecExitCode(ctx context.Context, ID string, execID string) (int, error) {
	ret := _m.Called(ctx, ID, execID)

	if len(ret) == 0 {
		panic("no return value specified for ExecExitCode")
	}

	var r0 int
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (int, error)); ok {
		return rf(ctx, ID, execID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) int); ok {
		r0 = rf(ctx, ID, execID)
	} else {
		r0 = ret.Get(0).(int)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, ID, execID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecResize provides a mock function with given fields: ctx, execID, height, width
func (_m *API) ExecResize(ctx context.Context, execID string, height uint, width uint) error {
	ret := _m.Called(ctx, execID, height, width)

	if len(ret) == 0 {
		panic("no return value specified for ExecResize")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, uint, uint) error); ok {
		r0 = rf(ctx, execID, height, width)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Execute provides a mock function with given fields: ctx, ID, config
func (_m *API) Execute(ctx context.Context, ID string, config *types.ExecConfig) (string, io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	ret := _m.Called(ctx, ID, config)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 string
	var r1 io.ReadCloser
	var r2 io.ReadCloser
	var r3 io.WriteCloser
	var r4 error
	if rf, ok := ret.Get(0).(func(context.Context, string, *types.ExecConfig) (string, io.ReadCloser, io.ReadCloser, io.WriteCloser, error)); ok {
		return rf(ctx, ID, config)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, *types.ExecConfig) string); ok {
		r0 = rf(ctx, ID, config)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, *types.ExecConfig) io.ReadCloser); ok {
		r1 = rf(ctx, ID, config)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, *types.ExecConfig) io.ReadCloser); ok {
		r2 = rf(ctx, ID, config)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(3).(func(context.Context, string, *types.ExecConfig) io.WriteCloser); ok {
		r3 = rf(ctx, ID, config)
	} else {
		if ret.Get(3) != nil {
			r3 = ret.Get(3).(io.WriteCloser)
		}
	}

	if rf, ok := ret.Get(4).(func(context.Context, string, *types.ExecConfig) error); ok {
		r4 = rf(ctx, ID, config)
	} else {
		r4 = ret.Error(4)
	}

	return r0, r1, r2, r3, r4
}

// GetParams provides a mock function with given fields:
func (_m *API) GetParams() *types.Params {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetParams")
	}

	var r0 *types.Params
	if rf, ok := ret.Get(0).(func() *types.Params); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Params)
		}
	}

	return r0
}

// ImageBuild provides a mock function with given fields: ctx, input, refs, platform
func (_m *API) ImageBuild(ctx context.Context, input io.Reader, refs []string, platform string) (io.ReadCloser, error) {
	ret := _m.Called(ctx, input, refs, platform)

	if len(ret) == 0 {
		panic("no return value specified for ImageBuild")
	}

	var r0 io.ReadCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, io.Reader, []string, string) (io.ReadCloser, error)); ok {
		return rf(ctx, input, refs, platform)
	}
	if rf, ok := ret.Get(0).(func(context.Context, io.Reader, []string, string) io.ReadCloser); ok {
		r0 = rf(ctx, input, refs, platform)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, io.Reader, []string, string) error); ok {
		r1 = rf(ctx, input, refs, platform)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImageBuildCachePrune provides a mock function with given fields: ctx, all
func (_m *API) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	ret := _m.Called(ctx, all)

	if len(ret) == 0 {
		panic("no return value specified for ImageBuildCachePrune")
	}

	var r0 uint64
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, bool) (uint64, error)); ok {
		return rf(ctx, all)
	}
	if rf, ok := ret.Get(0).(func(context.Context, bool) uint64); ok {
		r0 = rf(ctx, all)
	} else {
		r0 = ret.Get(0).(uint64)
	}

	if rf, ok := ret.Get(1).(func(context.Context, bool) error); ok {
		r1 = rf(ctx, all)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImageBuildFromExist provides a mock function with given fields: ctx, ID, refs, user
func (_m *API) ImageBuildFromExist(ctx context.Context, ID string, refs []string, user string) (string, error) {
	ret := _m.Called(ctx, ID, refs, user)

	if len(ret) == 0 {
		panic("no return value specified for ImageBuildFromExist")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, string) (string, error)); ok {
		return rf(ctx, ID, refs, user)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, []string, string) string); ok {
		r0 = rf(ctx, ID, refs, user)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, []string, string) error); ok {
		r1 = rf(ctx, ID, refs, user)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImageList provides a mock function with given fields: ctx, image
func (_m *API) ImageList(ctx context.Context, image string) ([]*types.Image, error) {
	ret := _m.Called(ctx, image)

	if len(ret) == 0 {
		panic("no return value specified for ImageList")
	}

	var r0 []*types.Image
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]*types.Image, error)); ok {
		return rf(ctx, image)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []*types.Image); ok {
		r0 = rf(ctx, image)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Image)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImageLocalDigests provides a mock function with given fields: ctx, image
func (_m *API) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	ret := _m.Called(ctx, image)

	if len(ret) == 0 {
		panic("no return value specified for ImageLocalDigests")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]string, error)); ok {
		return rf(ctx, image)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []string); ok {
		r0 = rf(ctx, image)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImagePull provides a mock function with given fields: ctx, ref, all
func (_m *API) ImagePull(ctx context.Context, ref string, all bool) (io.ReadCloser, error) {
	ret := _m.Called(ctx, ref, all)

	if len(ret) == 0 {
		panic("no return value specified for ImagePull")
	}

	var r0 io.ReadCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) (io.ReadCloser, error)); ok {
		return rf(ctx, ref, all)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, bool) io.ReadCloser); ok {
		r0 = rf(ctx, ref, all)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, bool) error); ok {
		r1 = rf(ctx, ref, all)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImagePush provides a mock function with given fields: ctx, ref
func (_m *API) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	ret := _m.Called(ctx, ref)

	if len(ret) == 0 {
		panic("no return value specified for ImagePush")
	}

	var r0 io.ReadCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (io.ReadCloser, error)); ok {
		return rf(ctx, ref)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) io.ReadCloser); ok {
		r0 = rf(ctx, ref)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, ref)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImageRemoteDigest provides a mock function with given fields: ctx, image
func (_m *API) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	ret := _m.Called(ctx, image)

	if len(ret) == 0 {
		panic("no return value specified for ImageRemoteDigest")
	}

	var r0 string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (string, error)); ok {
		return rf(ctx, image)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) string); ok {
		r0 = rf(ctx, image)
	} else {
		r0 = ret.Get(0).(string)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, image)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImageRemove provides a mock function with given fields: ctx, image, force, prune
func (_m *API) ImageRemove(ctx context.Context, image string, force bool, prune bool) ([]string, error) {
	ret := _m.Called(ctx, image, force, prune)

	if len(ret) == 0 {
		panic("no return value specified for ImageRemove")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool, bool) ([]string, error)); ok {
		return rf(ctx, image, force, prune)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, bool, bool) []string); ok {
		r0 = rf(ctx, image, force, prune)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, bool, bool) error); ok {
		r1 = rf(ctx, image, force, prune)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ImagesPrune provides a mock function with given fields: ctx
func (_m *API) ImagesPrune(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for ImagesPrune")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Info provides a mock function with given fields: ctx
func (_m *API) Info(ctx context.Context) (*types.Info, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Info")
	}

	var r0 *types.Info
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*types.Info, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *types.Info); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.Info)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NetworkConnect provides a mock function with given fields: ctx, network, target, ipv4, ipv6
func (_m *API) NetworkConnect(ctx context.Context, network string, target string, ipv4 string, ipv6 string) ([]string, error) {
	ret := _m.Called(ctx, network, target, ipv4, ipv6)

	if len(ret) == 0 {
		panic("no return value specified for NetworkConnect")
	}

	var r0 []string
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) ([]string, error)); ok {
		return rf(ctx, network, target, ipv4, ipv6)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, string, string) []string); ok {
		r0 = rf(ctx, network, target, ipv4, ipv6)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, string, string) error); ok {
		r1 = rf(ctx, network, target, ipv4, ipv6)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NetworkDisconnect provides a mock function with given fields: ctx, network, target, force
func (_m *API) NetworkDisconnect(ctx context.Context, network string, target string, force bool) error {
	ret := _m.Called(ctx, network, target, force)

	if len(ret) == 0 {
		panic("no return value specified for NetworkDisconnect")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, bool) error); ok {
		r0 = rf(ctx, network, target, force)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NetworkList provides a mock function with given fields: ctx, drivers
func (_m *API) NetworkList(ctx context.Context, drivers []string) ([]*types.Network, error) {
	ret := _m.Called(ctx, drivers)

	if len(ret) == 0 {
		panic("no return value specified for NetworkList")
	}

	var r0 []*types.Network
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []string) ([]*types.Network, error)); ok {
		return rf(ctx, drivers)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []string) []*types.Network); ok {
		r0 = rf(ctx, drivers)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*types.Network)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []string) error); ok {
		r1 = rf(ctx, drivers)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ping provides a mock function with given fields: ctx
func (_m *API) Ping(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Ping")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RawEngine provides a mock function with given fields: ctx, opts
func (_m *API) RawEngine(ctx context.Context, opts *types.RawEngineOptions) (*types.RawEngineResult, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for RawEngine")
	}

	var r0 *types.RawEngineResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.RawEngineOptions) (*types.RawEngineResult, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.RawEngineOptions) *types.RawEngineResult); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.RawEngineResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.RawEngineOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// VirtualizationAttach provides a mock function with given fields: ctx, ID, stream, openStdin
func (_m *API) VirtualizationAttach(ctx context.Context, ID string, stream bool, openStdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	ret := _m.Called(ctx, ID, stream, openStdin)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationAttach")
	}

	var r0 io.ReadCloser
	var r1 io.ReadCloser
	var r2 io.WriteCloser
	var r3 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool, bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error)); ok {
		return rf(ctx, ID, stream, openStdin)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, bool, bool) io.ReadCloser); ok {
		r0 = rf(ctx, ID, stream, openStdin)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, bool, bool) io.ReadCloser); ok {
		r1 = rf(ctx, ID, stream, openStdin)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, bool, bool) io.WriteCloser); ok {
		r2 = rf(ctx, ID, stream, openStdin)
	} else {
		if ret.Get(2) != nil {
			r2 = ret.Get(2).(io.WriteCloser)
		}
	}

	if rf, ok := ret.Get(3).(func(context.Context, string, bool, bool) error); ok {
		r3 = rf(ctx, ID, stream, openStdin)
	} else {
		r3 = ret.Error(3)
	}

	return r0, r1, r2, r3
}

// VirtualizationCopyChunkTo provides a mock function with given fields: ctx, ID, target, size, content, uid, gid, mode
func (_m *API) VirtualizationCopyChunkTo(ctx context.Context, ID string, target string, size int64, content io.Reader, uid int, gid int, mode int64) error {
	ret := _m.Called(ctx, ID, target, size, content, uid, gid, mode)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationCopyChunkTo")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, int64, io.Reader, int, int, int64) error); ok {
		r0 = rf(ctx, ID, target, size, content, uid, gid, mode)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationCopyFrom provides a mock function with given fields: ctx, ID, path
func (_m *API) VirtualizationCopyFrom(ctx context.Context, ID string, path string) ([]byte, int, int, int64, error) {
	ret := _m.Called(ctx, ID, path)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationCopyFrom")
	}

	var r0 []byte
	var r1 int
	var r2 int
	var r3 int64
	var r4 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) ([]byte, int, int, int64, error)); ok {
		return rf(ctx, ID, path)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) []byte); ok {
		r0 = rf(ctx, ID, path)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) int); ok {
		r1 = rf(ctx, ID, path)
	} else {
		r1 = ret.Get(1).(int)
	}

	if rf, ok := ret.Get(2).(func(context.Context, string, string) int); ok {
		r2 = rf(ctx, ID, path)
	} else {
		r2 = ret.Get(2).(int)
	}

	if rf, ok := ret.Get(3).(func(context.Context, string, string) int64); ok {
		r3 = rf(ctx, ID, path)
	} else {
		r3 = ret.Get(3).(int64)
	}

	if rf, ok := ret.Get(4).(func(context.Context, string, string) error); ok {
		r4 = rf(ctx, ID, path)
	} else {
		r4 = ret.Error(4)
	}

	return r0, r1, r2, r3, r4
}

// VirtualizationCopyTo provides a mock function with given fields: ctx, ID, target, content, uid, gid, mode
func (_m *API) VirtualizationCopyTo(ctx context.Context, ID string, target string, content []byte, uid int, gid int, mode int64) error {
	ret := _m.Called(ctx, ID, target, content, uid, gid, mode)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationCopyTo")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, []byte, int, int, int64) error); ok {
		r0 = rf(ctx, ID, target, content, uid, gid, mode)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationCreate provides a mock function with given fields: ctx, opts
func (_m *API) VirtualizationCreate(ctx context.Context, opts *types.VirtualizationCreateOptions) (*types.VirtualizationCreated, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationCreate")
	}

	var r0 *types.VirtualizationCreated
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.VirtualizationCreateOptions) (*types.VirtualizationCreated, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.VirtualizationCreateOptions) *types.VirtualizationCreated); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.VirtualizationCreated)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.VirtualizationCreateOptions) error); ok {
		r1 = rf(ctx, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// VirtualizationInspect provides a mock function with given fields: ctx, ID
func (_m *API) VirtualizationInspect(ctx context.Context, ID string) (*types.VirtualizationInfo, error) {
	ret := _m.Called(ctx, ID)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationInspect")
	}

	var r0 *types.VirtualizationInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (*types.VirtualizationInfo, error)); ok {
		return rf(ctx, ID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) *types.VirtualizationInfo); ok {
		r0 = rf(ctx, ID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.VirtualizationInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, ID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// VirtualizationLogs provides a mock function with given fields: ctx, opts
func (_m *API) VirtualizationLogs(ctx context.Context, opts *types.VirtualizationLogStreamOptions) (io.ReadCloser, io.ReadCloser, error) {
	ret := _m.Called(ctx, opts)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationLogs")
	}

	var r0 io.ReadCloser
	var r1 io.ReadCloser
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, *types.VirtualizationLogStreamOptions) (io.ReadCloser, io.ReadCloser, error)); ok {
		return rf(ctx, opts)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *types.VirtualizationLogStreamOptions) io.ReadCloser); ok {
		r0 = rf(ctx, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *types.VirtualizationLogStreamOptions) io.ReadCloser); ok {
		r1 = rf(ctx, opts)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(io.ReadCloser)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context, *types.VirtualizationLogStreamOptions) error); ok {
		r2 = rf(ctx, opts)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// VirtualizationRemove provides a mock function with given fields: ctx, ID, volumes, force
func (_m *API) VirtualizationRemove(ctx context.Context, ID string, volumes bool, force bool) error {
	ret := _m.Called(ctx, ID, volumes, force)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationRemove")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, bool, bool) error); ok {
		r0 = rf(ctx, ID, volumes, force)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationResize provides a mock function with given fields: ctx, ID, height, width
func (_m *API) VirtualizationResize(ctx context.Context, ID string, height uint, width uint) error {
	ret := _m.Called(ctx, ID, height, width)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationResize")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, uint, uint) error); ok {
		r0 = rf(ctx, ID, height, width)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationResume provides a mock function with given fields: ctx, ID
func (_m *API) VirtualizationResume(ctx context.Context, ID string) error {
	ret := _m.Called(ctx, ID)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationResume")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, ID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationStart provides a mock function with given fields: ctx, ID
func (_m *API) VirtualizationStart(ctx context.Context, ID string) error {
	ret := _m.Called(ctx, ID)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationStart")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, ID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationStop provides a mock function with given fields: ctx, ID, gracefulTimeout
func (_m *API) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	ret := _m.Called(ctx, ID, gracefulTimeout)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationStop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, time.Duration) error); ok {
		r0 = rf(ctx, ID, gracefulTimeout)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationSuspend provides a mock function with given fields: ctx, ID
func (_m *API) VirtualizationSuspend(ctx context.Context, ID string) error {
	ret := _m.Called(ctx, ID)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationSuspend")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, ID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationUpdateResource provides a mock function with given fields: ctx, ID, params
func (_m *API) VirtualizationUpdateResource(ctx context.Context, ID string, params resourcetypes.Resources) error {
	ret := _m.Called(ctx, ID, params)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationUpdateResource")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string, resourcetypes.Resources) error); ok {
		r0 = rf(ctx, ID, params)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// VirtualizationWait provides a mock function with given fields: ctx, ID, state
func (_m *API) VirtualizationWait(ctx context.Context, ID string, state string) (*types.VirtualizationWaitResult, error) {
	ret := _m.Called(ctx, ID, state)

	if len(ret) == 0 {
		panic("no return value specified for VirtualizationWait")
	}

	var r0 *types.VirtualizationWaitResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string) (*types.VirtualizationWaitResult, error)); ok {
		return rf(ctx, ID, state)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string) *types.VirtualizationWaitResult); ok {
		r0 = rf(ctx, ID, state)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*types.VirtualizationWaitResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string) error); ok {
		r1 = rf(ctx, ID, state)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewAPI creates a new instance of API. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *API {
	mock := &API{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
