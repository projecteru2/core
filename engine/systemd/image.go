package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
)

func (s *SystemdSSH) ImageList(ctx context.Context, image string) (images []*enginetypes.Image, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImageRemove(ctx context.Context, image string, force, prune bool) (layers []string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImagesPrune(ctx context.Context) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImagePull(ctx context.Context, ref string, all bool) (reader io.ReadCloser, err error) {
	return
}

func (s *SystemdSSH) ImagePush(ctx context.Context, ref string) (reader io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImageBuild(ctx context.Context, input io.Reader, refs []string) (reader io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImageBuildCachePrune(ctx context.Context, all bool) (reclaimedInBytes uint64, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImageLocalDigests(ctx context.Context, image string) (digests []string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) ImageRemoteDigest(ctx context.Context, image string) (digest string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) BuildRefs(ctx context.Context, name string, tags []string) (refs []string) {
	return
}

func (s *SystemdSSH) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildOptions) (dir string, reader io.Reader, err error) {
	err = types.ErrEngineNotImplemented
	return
}
