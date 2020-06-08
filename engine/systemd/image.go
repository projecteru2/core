package systemd

import (
	"context"
	"io"

	enginetypes "github.com/projecteru2/core/engine/types"
	coresource "github.com/projecteru2/core/source"
	"github.com/projecteru2/core/types"
)

// ImageList lists image
func (s *SSHClient) ImageList(ctx context.Context, image string) (images []*enginetypes.Image, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImageRemove removes image
func (s *SSHClient) ImageRemove(ctx context.Context, image string, force, prune bool) (layers []string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImagesPrune prunes
func (s *SSHClient) ImagesPrune(ctx context.Context) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImagePull pulls image
func (s *SSHClient) ImagePull(ctx context.Context, ref string, all bool) (ch chan *enginetypes.ImageMessage, err error) {
	return
}

// ImagePush pushes image
func (s *SSHClient) ImagePush(ctx context.Context, ref string) (ch chan *enginetypes.ImageMessage, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImageBuild builds image
func (s *SSHClient) ImageBuild(ctx context.Context, input io.Reader, refs []string) (reader io.ReadCloser, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImageBuildFromExist won't work for systemd engine
func (s *SSHClient) ImageBuildFromExist(ctx context.Context, ID, name string) (imageID string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImageBuildCachePrune prunes cache
func (s *SSHClient) ImageBuildCachePrune(ctx context.Context, all bool) (reclaimedInBytes uint64, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImageLocalDigests gets image local digest
func (s *SSHClient) ImageLocalDigests(ctx context.Context, image string) (digests []string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// ImageRemoteDigest gets image remote digest
func (s *SSHClient) ImageRemoteDigest(ctx context.Context, image string) (digest string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// BuildRefs builds images refs
func (s *SSHClient) BuildRefs(ctx context.Context, name string, tags []string) (refs []string) {
	return
}

// BuildContent builds image content
func (s *SSHClient) BuildContent(ctx context.Context, scm coresource.Source, opts *enginetypes.BuildContentOptions) (dir string, reader io.Reader, err error) {
	err = types.ErrEngineNotImplemented
	return
}
