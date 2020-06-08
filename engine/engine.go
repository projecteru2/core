package engine

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
)

// API define a remote engine
type API interface {
	Info(ctx context.Context) (*enginetypes.Info, error)

	ExecCreate(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, error)
	ExecAttach(ctx context.Context, execID string, tty bool) (io.ReadCloser, io.WriteCloser, error)
	Execute(ctx context.Context, target string, config *enginetypes.ExecConfig) (string, io.ReadCloser, io.WriteCloser, error)
	ExecResize(ctx context.Context, execID string, height, width uint) (err error)
	ExecExitCode(ctx context.Context, execID string) (int, error)

	NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) error
	NetworkDisconnect(ctx context.Context, network, target string, force bool) error
	NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error)

	ImageList(ctx context.Context, image string) ([]*enginetypes.Image, error)
	ImageRemove(ctx context.Context, image string, force, prune bool) ([]string, error)
	ImagesPrune(ctx context.Context) error
	ImagePull(ctx context.Context, ref string, all bool) (chan *enginetypes.ImageMessage, error)
	ImagePush(ctx context.Context, ref string) (chan *enginetypes.ImageMessage, error)
	ImageBuild(ctx context.Context, input io.Reader, refs []string) (io.ReadCloser, error)
	ImageBuildCachePrune(ctx context.Context, all bool) (uint64, error)
	ImageLocalDigests(ctx context.Context, image string) ([]string, error)
	ImageRemoteDigest(ctx context.Context, image string) (string, error)
	ImageBuildFromExist(ctx context.Context, ID, name string) (string, error)

	BuildRefs(ctx context.Context, name string, tags []string) []string
	BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (string, io.Reader, error)

	VirtualizationCreate(ctx context.Context, opts *enginetypes.VirtualizationCreateOptions) (*enginetypes.VirtualizationCreated, error)
	VirtualizationCopyTo(ctx context.Context, ID, target string, content io.Reader, AllowOverwriteDirWithFile, CopyUIDGID bool) error
	VirtualizationStart(ctx context.Context, ID string) error
	VirtualizationStop(ctx context.Context, ID string) error
	VirtualizationRemove(ctx context.Context, ID string, volumes, force bool) error
	VirtualizationInspect(ctx context.Context, ID string) (*enginetypes.VirtualizationInfo, error)
	VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (io.ReadCloser, error)
	VirtualizationAttach(ctx context.Context, ID string, stream, stdin bool) (io.ReadCloser, io.WriteCloser, error)
	VirtualizationResize(ctx context.Context, ID string, height, width uint) error
	VirtualizationWait(ctx context.Context, ID, state string) (*enginetypes.VirtualizationWaitResult, error)
	VirtualizationUpdateResource(ctx context.Context, ID string, opts *enginetypes.VirtualizationResource) error
	VirtualizationCopyFrom(ctx context.Context, ID, path string) (io.ReadCloser, string, error)

	ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) error
}
