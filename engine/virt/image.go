package virt

import (
	"context"
	"io"

	log "github.com/sirupsen/logrus"

	enginetypes "github.com/projecteru2/core/engine/types"
)

// ImageList lists images.
func (v *Virt) ImageList(ctx context.Context, image string) (imgs []*enginetypes.Image, err error) {
	log.Warnf("does not implement")
	return
}

// ImageRemove removes a specific image.
func (v *Virt) ImageRemove(ctx context.Context, image string, force, prune bool) (names []string, err error) {
	log.Warnf("does not implement")
	return
}

// ImagesPrune prunes one.
func (v *Virt) ImagesPrune(ctx context.Context) (err error) {
	log.Warnf("does not implement")
	return
}

// ImagePull pulls an image to local virt-node.
func (v *Virt) ImagePull(ctx context.Context, ref string, all bool) (stream io.ReadCloser, err error) {
	return
}

// ImagePush pushes to central image registry.
func (v *Virt) ImagePush(ctx context.Context, ref string) (rc io.ReadCloser, err error) {
	log.Warnf("does not implement")
	return
}

// ImageBuild captures from a guest.
func (v *Virt) ImageBuild(ctx context.Context, input io.Reader, refs []string) (rc io.ReadCloser, err error) {
	log.Warnf("does not implement")
	return
}

// ImageBuildFromExist builds vm image from running vm
func (v *Virt) ImageBuildFromExist(ctx context.Context, ID, name string) (imageID string, err error) {
	log.Warnf("does not implement")
	return
}

// ImageBuildCachePrune prunes cached one.
func (v *Virt) ImageBuildCachePrune(ctx context.Context, all bool) (reclaimed uint64, err error) {
	log.Warnf("does not implement")
	return
}

// ImageLocalDigests shows local images' digests.
func (v *Virt) ImageLocalDigests(ctx context.Context, image string) (digests []string, err error) {
	return
}

// ImageRemoteDigest shows remote one's digest.
func (v *Virt) ImageRemoteDigest(ctx context.Context, image string) (digest string, err error) {
	return
}
