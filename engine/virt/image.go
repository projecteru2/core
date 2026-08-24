package virt

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"

	virttypes "github.com/projecteru2/libyavirt/types"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

func (v *Virt) ImageList(ctx context.Context, imageName string) (imgs []*enginetypes.Image, err error) {
	images, err := v.client.ListImage(ctx, imageName)
	if err != nil {
		return nil, err
	}

	imgs = []*enginetypes.Image{}

	for _, image := range images {
		imgs = append(imgs, &enginetypes.Image{
			ID:   image.ID,
			Tags: []string{image.Name},
		})
	}

	return imgs, err
}

func (v *Virt) ImageRemove(ctx context.Context, tag string, force, prune bool) (names []string, err error) {
	user, imgName, err := splitUserImage(tag)
	if err != nil {
		return nil, err
	}

	return v.client.RemoveImage(ctx, imgName, user, force, prune)
}

func (v *Virt) ImagesPrune(context.Context) error {
	return types.ErrEngineNotImplemented
}

func (v *Virt) ImagePull(ctx context.Context, ref string, all bool) (rc io.ReadCloser, err error) {
	// yavirt pulls by image name only, the user part is discarded
	_, imgName, err := splitUserImage(ref)
	if err != nil {
		return nil, err
	}

	msg, err := v.client.PullImage(ctx, imgName, all)
	if err != nil {
		return nil, err
	}

	return io.NopCloser(strings.NewReader(msg)), nil
}

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

	return io.NopCloser(bytes.NewReader(reply)), nil
}

func (v *Virt) ImageBuild(context.Context, io.Reader, []string, string) (io.ReadCloser, error) {
	return nil, types.ErrEngineNotImplemented
}

func (v *Virt) ImageBuildFromExist(ctx context.Context, ID string, refs []string, user string) (string, error) {
	if len(user) < 1 {
		return "", types.ErrNoImageUser
	}
	if len(refs) != 1 {
		return "", types.ErrInvaildRefs
	}

	_, imgName, err := splitUserImage(refs[0])
	if err != nil {
		return "", err
	}

	req := virttypes.CaptureGuestReq{Name: imgName, User: user, ID: ID}

	uimg, err := v.client.CaptureGuest(ctx, req)
	if err != nil {
		return "", err
	}

	return uimg.ID, nil
}

func (v *Virt) ImageBuildCachePrune(context.Context, bool) (uint64, error) {
	return 0, types.ErrEngineNotImplemented
}

func (v *Virt) ImageLocalDigests(ctx context.Context, image string) ([]string, error) {
	_, imgName, err := splitUserImage(image)
	if err != nil {
		return nil, err
	}

	return v.client.DigestImage(ctx, imgName, true)
}

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
		return "", types.ErrInvaildRemoteDigest
	default:
		return digests[0], nil
	}
}
