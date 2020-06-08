package engine

import (
	"context"
	"io"
	"time"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
)

// TimeoutAPI wraps engine implementation with context.WithTimeout
type TimeoutAPI struct {
	api     API
	timeout time.Duration
}

// WithGlobalTimeout .
func WithGlobalTimeout(api API, timeout time.Duration) API {
	return &TimeoutAPI{
		api:     api,
		timeout: timeout,
	}
}

// Info .
func (a *TimeoutAPI) Info(ctx context.Context) (*enginetypes.Info, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.Info(ctx)
}

// ExecCreate .
func (a *TimeoutAPI) ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ExecCreate(ctx, target, config)
}

// ExecAttach .
func (a *TimeoutAPI) ExecAttach(ctx context.Context, execID string, tty bool) (io.ReadCloser, io.WriteCloser, error) {
	return a.api.ExecAttach(ctx, execID, tty)
}

// Execute .
func (a *TimeoutAPI) Execute(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, io.ReadCloser, io.WriteCloser, error) {
	return a.api.Execute(ctx, target, config)
}

// ExecResize .
func (a *TimeoutAPI) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ExecResize(ctx, execID, height, width)
}

// ExecExitCode .
func (a *TimeoutAPI) ExecExitCode(ctx context.Context, execID string) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ExecExitCode(ctx, execID)
}

// NetworkConnect .
func (a *TimeoutAPI) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.NetworkConnect(ctx, network, target, ipv4, ipv6)
}

// NetworkDisconnect .
func (a *TimeoutAPI) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.NetworkDisconnect(ctx, network, target, force)
}

// NetworkList .
func (a *TimeoutAPI) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.NetworkList(ctx, drivers)
}

// ImageList .
func (a *TimeoutAPI) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageList(ctx, image)
}

// ImageRemove .
func (a *TimeoutAPI) ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageRemove(ctx, image, force, prune)
}

// ImagesPrune .
func (a *TimeoutAPI) ImagesPrune(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImagesPrune(ctx)
}

// ImagePull .
func (a *TimeoutAPI) ImagePull(ctx context.Context, ref string, all bool) (chan *enginetypes.ImageMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImagePull(ctx, ref, all)
}

// ImagePush .
func (a *TimeoutAPI) ImagePush(ctx context.Context, ref string) (chan *enginetypes.ImageMessage, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImagePush(ctx, ref)
}

// ImageBuild .
func (a *TimeoutAPI) ImageBuild(ctx context.Context, input io.Reader, refs []string) (io.ReadCloser, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageBuild(ctx, input, refs)
}

// ImageBuildCachePrune .
func (a *TimeoutAPI) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageBuildCachePrune(ctx, all)
}

// ImageLocalDigests .
func (a *TimeoutAPI) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageLocalDigests(ctx, image)
}

// ImageRemoteDigest .
func (a *TimeoutAPI) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageRemoteDigest(ctx, image)
}

// ImageBuildFromExist .
func (a *TimeoutAPI) ImageBuildFromExist(ctx context.Context, ID, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ImageBuildFromExist(ctx, ID, name)
}

// BuildRefs .
func (a *TimeoutAPI) BuildRefs(ctx context.Context, name string, tags []string) []string {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.BuildRefs(ctx, name, tags)
}

// BuildContent .
func (a *TimeoutAPI) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.BuildContent(ctx, scm, opts)
}

// VirtualizationCreate .
func (a *TimeoutAPI) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationCreate(ctx, opts)
}

// VirtualizationCopyTo .
func (a *TimeoutAPI) VirtualizationCopyTo(ctx context.Context, ID, target string, content io.Reader, AllowOverwriteDirWithFile, CopyUIDGID bool) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationCopyTo(ctx, ID, target, content, AllowOverwriteDirWithFile, CopyUIDGID)
}

// VirtualizationStart .
func (a *TimeoutAPI) VirtualizationStart(ctx context.Context, ID string) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationStart(ctx, ID)
}

// VirtualizationStop .
func (a *TimeoutAPI) VirtualizationStop(ctx context.Context, ID string) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationStop(ctx, ID)
}

// VirtualizationRemove .
func (a *TimeoutAPI) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationRemove(ctx, ID, volumes, force)
}

// VirtualizationInspect .
func (a *TimeoutAPI) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationInspect(ctx, ID)
}

// VirtualizationLogs .
func (a *TimeoutAPI) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (io.ReadCloser, error) {
	return a.api.VirtualizationLogs(ctx, opts)
}

// VirtualizationAttach .
func (a *TimeoutAPI) VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (io.ReadCloser, io.WriteCloser, error) {
	return a.api.VirtualizationAttach(ctx, ID, stream, stdin)
}

// VirtualizationResize .
func (a *TimeoutAPI) VirtualizationResize(ctx context.Context, ID string, height, width uint) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationResize(ctx, ID, height, width)
}

// VirtualizationWait .
func (a *TimeoutAPI) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationWait(ctx, ID, state)
}

// VirtualizationUpdateResource .
func (a *TimeoutAPI) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationUpdateResource(ctx, ID, opts)
}

// VirtualizationCopyFrom .
func (a *TimeoutAPI) VirtualizationCopyFrom(ctx context.Context, ID, path string) (io.ReadCloser, string, error) {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.VirtualizationCopyFrom(ctx, ID, path)
}

// ResourceValidate .
func (a *TimeoutAPI) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error {
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()
	return a.api.ResourceValidate(ctx, cpu, cpumap, memory, storage)
}
