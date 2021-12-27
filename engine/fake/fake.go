package fake

import (
	"context"
	"io"
	"time"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
)

// Engine to replace nil engine
type Engine struct{}

// Info .
func (f *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	return nil, types.ErrNilEngine
}

// Execute .
func (f *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (result string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return "", nil, nil, nil, types.ErrNilEngine
}

// ExecResize .
func (f *Engine) ExecResize(ctx context.Context, ID, result string, height, width uint) (err error) {
	return types.ErrNilEngine
}

// ExecExitCode .
func (f *Engine) ExecExitCode(ctx context.Context, ID, result string) (int, error) {
	return 0, types.ErrNilEngine
}

// NetworkConnect .
func (f *Engine) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) ([]string, error) {
	return nil, types.ErrNilEngine
}

// NetworkDisconnect .
func (f *Engine) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	return types.ErrNilEngine
}

// NetworkList .
func (f *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	return nil, types.ErrNilEngine
}

// ImageList .
func (f *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	return nil, types.ErrNilEngine
}

// ImageRemove .
func (f *Engine) ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error) {
	return nil, types.ErrNilEngine
}

// ImagesPrune .
func (f *Engine) ImagesPrune(ctx context.Context) error {
	return types.ErrNilEngine
}

// ImagePull .
func (f *Engine) ImagePull(ctx context.Context, ref string, all bool) (io.ReadCloser, error) {
	return nil, types.ErrNilEngine
}

// ImagePush .
func (f *Engine) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	return nil, types.ErrNilEngine
}

// ImageBuild .
func (f *Engine) ImageBuild(ctx context.Context, input io.Reader, refs []string) (io.ReadCloser, error) {
	return nil, types.ErrNilEngine
}

// ImageBuildCachePrune .
func (f *Engine) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	return 0, types.ErrNilEngine
}

// ImageLocalDigests .
func (f *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	return nil, types.ErrNilEngine
}

// ImageRemoteDigest .
func (f *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	return "", types.ErrNilEngine
}

// ImageBuildFromExist .
func (f *Engine) ImageBuildFromExist(ctx context.Context, ID string, refs []string, user string) (string, error) {
	return "", types.ErrNilEngine
}

// BuildRefs .
func (f *Engine) BuildRefs(ctx context.Context, opts *enginetypes.BuildRefOptions) []string {
	return nil
}

// BuildContent .
func (f *Engine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, types.ErrNilEngine
}

// VirtualizationCreate .
func (f *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	return nil, types.ErrNilEngine
}

// VirtualizationResourceRemap .
func (f *Engine) VirtualizationResourceRemap(ctx context.Context, options *enginetypes.VirtualizationRemapOptions) (<-chan enginetypes.VirtualizationRemapMessage, error) {
	return nil, types.ErrNilEngine
}

// VirtualizationCopyTo .
func (f *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return types.ErrNilEngine
}

// VirtualizationStart .
func (f *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	return types.ErrNilEngine
}

// VirtualizationStop .
func (f *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	return types.ErrNilEngine
}

// VirtualizationRemove .
func (f *Engine) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) error {
	return types.ErrNilEngine
}

// VirtualizationInspect .
func (f *Engine) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	return nil, types.ErrNilEngine
}

// VirtualizationLogs .
func (f *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	return nil, nil, types.ErrNilEngine
}

// VirtualizationAttach .
func (f *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, openStdin bool) (stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return nil, nil, nil, types.ErrNilEngine
}

// VirtualizationResize .
func (f *Engine) VirtualizationResize(ctx context.Context, ID string, height, width uint) error {
	return types.ErrNilEngine
}

// VirtualizationWait .
func (f *Engine) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	return nil, types.ErrNilEngine
}

// VirtualizationUpdateResource .
func (f *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	return types.ErrNilEngine
}

// VirtualizationCopyFrom .
func (f *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, _ error) {
	return nil, 0, 0, 0, types.ErrNilEngine
}

// ResourceValidate .
func (f *Engine) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error {
	return types.ErrNilEngine
}
