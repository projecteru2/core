package fake

import (
	"context"
	"io"
	"time"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
)

// EngineWithErr use to mock the nil engine
type EngineWithErr struct {
	DefaultErr error
}

// Info .
func (f *EngineWithErr) Info(ctx context.Context) (*enginetypes.Info, error) {
	return nil, f.DefaultErr
}

// Ping .
func (f *EngineWithErr) Ping(ctx context.Context) error {
	return f.DefaultErr
}

// CloseConn .
func (f *EngineWithErr) CloseConn() error {
	return nil
}

// Execute .
func (f *EngineWithErr) Execute(ctx context.Context, ID string, config *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return "", nil, nil, nil, f.DefaultErr
}

// ExecResize .
func (f *EngineWithErr) ExecResize(ctx context.Context, execID string, height, width uint) (err error) {
	return f.DefaultErr
}

// ExecExitCode .
func (f *EngineWithErr) ExecExitCode(ctx context.Context, ID, result string) (int, error) {
	return 0, f.DefaultErr
}

// NetworkConnect .
func (f *EngineWithErr) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) ([]string, error) {
	return nil, f.DefaultErr
}

// NetworkDisconnect .
func (f *EngineWithErr) NetworkDisconnect(ctx context.Context, network, target string, force bool) error {
	return f.DefaultErr
}

// NetworkList .
func (f *EngineWithErr) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	return nil, f.DefaultErr
}

// ImageList .
func (f *EngineWithErr) ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error) {
	return nil, f.DefaultErr
}

// ImageRemove .
func (f *EngineWithErr) ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error) {
	return nil, f.DefaultErr
}

// ImagesPrune .
func (f *EngineWithErr) ImagesPrune(ctx context.Context) error {
	return f.DefaultErr
}

// ImagePull .
func (f *EngineWithErr) ImagePull(ctx context.Context, ref string, all bool) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

// ImagePush .
func (f *EngineWithErr) ImagePush(ctx context.Context, ref string) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

// ImageBuild .
func (f *EngineWithErr) ImageBuild(ctx context.Context, input io.Reader, refs []string, _ string) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

// ImageBuildCachePrune .
func (f *EngineWithErr) ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error) {
	return 0, f.DefaultErr
}

// ImageLocalDigests .
func (f *EngineWithErr) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	return nil, f.DefaultErr
}

// ImageRemoteDigest .
func (f *EngineWithErr) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	return "", f.DefaultErr
}

// ImageBuildFromExist .
func (f *EngineWithErr) ImageBuildFromExist(ctx context.Context, ID string, refs []string, user string) (string, error) {
	return "", f.DefaultErr
}

// BuildRefs .
func (f *EngineWithErr) BuildRefs(ctx context.Context, opts *enginetypes.BuildRefOptions) []string {
	return nil
}

// BuildContent .
func (f *EngineWithErr) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, f.DefaultErr
}

// VirtualizationCreate .
func (f *EngineWithErr) VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	return nil, f.DefaultErr
}

// VirtualizationResourceRemap .
func (f *EngineWithErr) VirtualizationResourceRemap(ctx context.Context, options *enginetypes.VirtualizationRemapOptions) (<-chan enginetypes.VirtualizationRemapMessage, error) {
	return nil, f.DefaultErr
}

// VirtualizationCopyTo .
func (f *EngineWithErr) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return f.DefaultErr
}

// VirtualizationStart .
func (f *EngineWithErr) VirtualizationStart(ctx context.Context, ID string) error {
	return f.DefaultErr
}

// VirtualizationStop .
func (f *EngineWithErr) VirtualizationStop(ctx context.Context, ID string, gracefulTimeout time.Duration) error {
	return f.DefaultErr
}

// VirtualizationRemove .
func (f *EngineWithErr) VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) error {
	return f.DefaultErr
}

// VirtualizationInspect .
func (f *EngineWithErr) VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error) {
	return nil, f.DefaultErr
}

// VirtualizationLogs .
func (f *EngineWithErr) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	return nil, nil, f.DefaultErr
}

// VirtualizationAttach .
func (f *EngineWithErr) VirtualizationAttach(ctx context.Context, ID string, stream, openStdin bool) (stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return nil, nil, nil, f.DefaultErr
}

// VirtualizationResize .
func (f *EngineWithErr) VirtualizationResize(ctx context.Context, ID string, height, width uint) error {
	return f.DefaultErr
}

// VirtualizationWait .
func (f *EngineWithErr) VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error) {
	return nil, f.DefaultErr
}

// VirtualizationUpdateResource .
func (f *EngineWithErr) VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error {
	return f.DefaultErr
}

// VirtualizationCopyFrom .
func (f *EngineWithErr) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, _ error) {
	return nil, 0, 0, 0, f.DefaultErr
}
