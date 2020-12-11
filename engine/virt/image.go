package virt

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/projecteru2/core/log"

	enginetypes "github.com/projecteru2/core/engine/types"

	virttypes "github.com/projecteru2/libyavirt/types"
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
func (v *Virt) ImagePull(ctx context.Context, ref string, all bool) (rc io.ReadCloser, err error) {
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
func (v *Virt) ImageBuildFromExist(ctx context.Context, ID, name string) (string, error) {
	// TODO: removes below 2 lines
	// upper layer may remove 'hub.docker.io/...../<name>' prefix and tag from the name.
	// due to the domain and tag both are docker concepts.
	// Removes domain part.
	name = filepath.Base(name)
	// Removes tag (latest by default)
	name = strings.Split(name, ":")[0]

	req := virttypes.CaptureGuestReq{Name: name}
	req.ID = ID

	uimg, err := v.client.CaptureGuest(ctx, req)
	if err != nil {
		return "", err
	}

	return uimg.Name, nil
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
