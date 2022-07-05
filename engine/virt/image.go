package virt

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	virttypes "github.com/projecteru2/libyavirt/types"
)

// ImageList lists images.
func (v *Virt) ImageList(ctx context.Context, imageName string) (imgs []*enginetypes.Image, err error) {
	images, err := v.client.ListImage(ctx, imageName)
	if err != nil {
		return nil, err
	}

	imgs = []*enginetypes.Image{}

	for _, image := range images {
		imgs = append(imgs, &enginetypes.Image{
			ID:   image.Id,
			Tags: []string{image.Name},
		})
	}

	return
}

// ImageRemove removes a specific image.
func (v *Virt) ImageRemove(ctx context.Context, tag string, force, prune bool) (names []string, err error) {
	user, imgName, err := splitUserImage(tag)
	if err != nil {
		return nil, err
	}

	return v.client.RemoveImage(ctx, imgName, user, force, prune)
}

// ImagesPrune prunes one.
func (v *Virt) ImagesPrune(ctx context.Context) (err error) {
	log.Warnf(ctx, "ImagesPrune does not implement")
	return
}

// ImagePull pulls an image to local virt-node.
func (v *Virt) ImagePull(ctx context.Context, ref string, all bool) (rc io.ReadCloser, err error) {
	// ref is a simple image name without username for now
	_, imgName, err := splitUserImage(ref)
	if err != nil {
		return nil, err
	}

	msg, err := v.client.PullImage(ctx, imgName, all)
	if err != nil {
		return nil, err
	}

	rc = io.NopCloser(strings.NewReader(msg))
	defer rc.Close()

	return rc, err
}

// ImagePush pushes to central image registry.
func (v *Virt) ImagePush(ctx context.Context, ref string) (rc io.ReadCloser, err error) {
	user, imgName, err := splitUserImage(ref)
	if err != nil {
		return nil, err
	}

	msg, err := v.client.PushImage(ctx, imgName, user)
	if err != nil {
		return nil, err
	}

	reply, err := json.Marshal(&types.BuildImageMessage{Error: msg})
	if err != nil {
		return nil, err
	}

	rc = io.NopCloser(bytes.NewReader(reply))
	defer rc.Close()

	return rc, nil
}

// ImageBuild captures from a guest.
func (v *Virt) ImageBuild(ctx context.Context, input io.Reader, refs []string) (rc io.ReadCloser, err error) {
	log.Warnf(ctx, "imageBuild does not implement")
	return
}

// ImageBuildFromExist builds vm image from running vm
func (v *Virt) ImageBuildFromExist(ctx context.Context, ID string, refs []string, user string) (string, error) {
	if len(user) < 1 {
		return "", types.ErrNoImageUser
	}
	if len(refs) != 1 {
		return "", types.ErrBadRefs
	}

	_, imgName, err := splitUserImage(refs[0])
	if err != nil {
		return "", err
	}

	req := virttypes.CaptureGuestReq{Name: imgName, User: user}
	req.ID = ID

	uimg, err := v.client.CaptureGuest(ctx, req)
	if err != nil {
		return "", err
	}

	return uimg.ID, nil
}

// ImageBuildCachePrune prunes cached one.
func (v *Virt) ImageBuildCachePrune(ctx context.Context, all bool) (reclaimed uint64, err error) {
	log.Warnf(ctx, "ImageBuildCachePrune does not implement and not required by vm")
	return
}

// ImageLocalDigests shows local images' digests.
// If local image file not exists return error
// If exists return digests
// Same for remote digest
func (v *Virt) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	// If not exists return error
	// If exists return digests
	_, imgName, err := splitUserImage(image)
	if err != nil {
		return nil, err
	}

	return v.client.DigestImage(ctx, imgName, true)
}

// ImageRemoteDigest shows remote one's digest.
func (v *Virt) ImageRemoteDigest(ctx context.Context, image string) (string, error) {
	_, imgName, err := splitUserImage(image)
	if err != nil {
		return "", err
	}

	digests, err := v.client.DigestImage(ctx, imgName, false)
	switch {
	case err != nil:
		return "", err
	case len(digests) < 1:
		return "", types.ErrNoRemoteDigest
	default:
		return digests[0], nil
	}
}
