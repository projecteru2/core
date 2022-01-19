package fake

import (
	"context"
	"io"
	"time"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
)

// Engine to replace nil engine
type Engine struct {
	DefaultErr error
}

// Info .
func (f *Engine) Info(ctx context.Context) (*enginetypes.Info, error) {
	return nil, f.DefaultErr
}

// Ping .
func (f *Engine) Ping(ctx context.Context) error {
	return f.DefaultErr
}

// Execute .
func (f *Engine) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (result string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return "", nil, nil, nil, f.DefaultErr
}

// ExecResize .
func (f *Engine) ExecResize(ctx context.Context, ID, result string, height, width uint) (err error) {
	return f.DefaultErr
}

// ExecExitCode .
func (f *Engine) ExecExitCode(ctx context.Context, ID, result string) (int, error) {
	return 0, f.DefaultErr
}

// NetworkConnect .
func (f *Engine) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) ([]string, error) {
	return nil, f.DefaultErr
}

// NetworkDisconnect .
func (f *Engine) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	return f.DefaultErr
}

// NetworkList .
func (f *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	return nil, f.DefaultErr
}

// ImageList .
func (f *Engine) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	return nil, f.DefaultErr
}

// ImageRemove .
func (f *Engine) ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error) {
	return nil, f.DefaultErr
}

// ImagesPrune .
func (f *Engine) ImagesPrune(ctx context.Context) error {
	return f.DefaultErr
}

// ImagePull .
func (f *Engine) ImagePull(ctx context.Context, ref string, all bool) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

// ImagePush .
func (f *Engine) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

// ImageBuild .
func (f *Engine) ImageBuild(ctx context.Context, input io.Reader, refs []string) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

// ImageBuildCachePrune .
func (f *Engine) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	return 0, f.DefaultErr
}

// ImageLocalDigests .
func (f *Engine) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	return nil, f.DefaultErr
}

// ImageRemoteDigest .
func (f *Engine) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	return "", f.DefaultErr
}

// ImageBuildFromExist .
func (f *Engine) ImageBuildFromExist(ctx context.Context, ID string, refs []string, user string) (string, error) {
	return "", f.DefaultErr
}

// BuildRefs .
func (f *Engine) BuildRefs(ctx context.Context, opts *enginetypes.BuildRefOptions) []string {
	return nil
}

// BuildContent .
func (f *Engine) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, f.DefaultErr
}

// VirtualizationCreate .
func (f *Engine) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	return nil, f.DefaultErr
}

// VirtualizationResourceRemap .
func (f *Engine) VirtualizationResourceRemap(ctx context.Context, options *enginetypes.VirtualizationRemapOptions) (<-chan enginetypes.VirtualizationRemapMessage, error) {
	return nil, f.DefaultErr
}

// VirtualizationCopyTo .
func (f *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return f.DefaultErr
}

// VirtualizationStart .
func (f *Engine) VirtualizationStart(ctx context.Context, ID string) error {
	return f.DefaultErr
}

// VirtualizationStop .
func (f *Engine) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	return f.DefaultErr
}

// VirtualizationRemove .
func (f *Engine) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) error {
	return f.DefaultErr
}

// VirtualizationInspect .
func (f *Engine) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	return nil, f.DefaultErr
}

// VirtualizationLogs .
func (f *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	return nil, nil, f.DefaultErr
}

// VirtualizationAttach .
func (f *Engine) VirtualizationAttach(ctx context.Context, ID string, stream, openStdin bool) (stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return nil, nil, nil, f.DefaultErr
}

// VirtualizationResize .
func (f *Engine) VirtualizationResize(ctx context.Context, ID string, height, width uint) error {
	return f.DefaultErr
}

// VirtualizationWait .
func (f *Engine) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	return nil, f.DefaultErr
}

// VirtualizationUpdateResource .
func (f *Engine) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	return f.DefaultErr
}

// VirtualizationCopyFrom .
func (f *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, _ error) {
	return nil, 0, 0, 0, f.DefaultErr
}

// ResourceValidate .
func (f *Engine) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error {
	return f.DefaultErr
}
