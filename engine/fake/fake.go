package fake

import (
	"context"
	"io"
	"time"

	enginetypes "github.com/projecteru2/core/engine/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coresource "github.com/projecteru2/core/source"
)

// EngineWithErr is the placeholder engine stored for a node whose real engine cannot be built.
type EngineWithErr struct {
	DefaultErr error
	EP         *enginetypes.Params
}

func (f *EngineWithErr) Info(_ context.Context) (*enginetypes.Info, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) Ping(_ context.Context) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VerifyNode(_ context.Context) error {
	return f.DefaultErr
}

func (f *EngineWithErr) CloseConn() error {
	return nil
}

func (f *EngineWithErr) GetParams() *enginetypes.Params {
	return f.EP
}

func (f *EngineWithErr) Execute(context.Context, string, *enginetypes.ExecConfig) (execID string, stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return "", nil, nil, nil, f.DefaultErr
}

func (f *EngineWithErr) ExecResize(context.Context, string, uint, uint) (err error) {
	return f.DefaultErr
}

func (f *EngineWithErr) ExecExitCode(context.Context, string, string) (int, error) {
	return 0, f.DefaultErr
}

func (f *EngineWithErr) NetworkConnect(context.Context, string, string, string, string) ([]string, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) NetworkDisconnect(context.Context, string, string, bool) error {
	return f.DefaultErr
}

func (f *EngineWithErr) NetworkList(context.Context, []string) ([]*enginetypes.Network, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImageList(context.Context, string) ([]*enginetypes.Image, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImageRemove(context.Context, string, bool, bool) ([]string, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImagesPrune(context.Context) error {
	return f.DefaultErr
}

func (f *EngineWithErr) ImagePull(context.Context, string, bool) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImagePush(context.Context, string) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImageBuild(context.Context, io.Reader, []string, string) (io.ReadCloser, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImageBuildCachePrune(context.Context, bool) (uint64, error) {
	return 0, f.DefaultErr
}

func (f *EngineWithErr) ImageLocalDigests(context.Context, string) ([]string, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) ImageRemoteDigest(context.Context, string) (string, error) {
	return "", f.DefaultErr
}

func (f *EngineWithErr) ImageBuildFromExist(context.Context, string, []string, string) (string, error) {
	return "", f.DefaultErr
}

func (f *EngineWithErr) BuildRefs(context.Context, *enginetypes.BuildRefOptions) []string {
	return nil
}

func (f *EngineWithErr) BuildContent(context.Context, coresource.Source, *enginetypes.BuildContentOptions) (string, io.Reader, error) {
	return "", nil, f.DefaultErr
}

func (f *EngineWithErr) VirtualizationCreate(context.Context, *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) VirtualizationCopyTo(context.Context, string, string, []byte, int, int, int64) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationCopyChunkTo(context.Context, string, string, int64, io.Reader, int, int, int64) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationStart(context.Context, string) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationStop(context.Context, string, time.Duration) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationSuspend(context.Context, string) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationResume(context.Context, string) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationRemove(context.Context, string, bool, bool) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationInspect(context.Context, string) (*enginetypes.VirtualizationInfo, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) VirtualizationLogs(context.Context, *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	return nil, nil, f.DefaultErr
}

func (f *EngineWithErr) VirtualizationAttach(context.Context, string, bool, bool) (stdout, stderr io.ReadCloser, stdin io.WriteCloser, err error) {
	return nil, nil, nil, f.DefaultErr
}

func (f *EngineWithErr) VirtualizationResize(context.Context, string, uint, uint) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationWait(context.Context, string, string) (*enginetypes.VirtualizationWaitResult, error) {
	return nil, f.DefaultErr
}

func (f *EngineWithErr) VirtualizationUpdateResource(context.Context, string, resourcetypes.Resources) error {
	return f.DefaultErr
}

func (f *EngineWithErr) VirtualizationCopyFrom(context.Context, string, string) (content []byte, uid, gid int, mode int64, _ error) {
	return nil, 0, 0, 0, f.DefaultErr
}

func (f *EngineWithErr) RawEngine(context.Context, *enginetypes.RawEngineOptions) (*enginetypes.RawEngineResult, error) {
	return nil, f.DefaultErr
}
