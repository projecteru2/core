package virt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"

	virttypes "github.com/projecteru2/libyavirt/types"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// ImageList lists images.
func (v *Virt) ImageList(ctx context.Context, image string) (imgs []*enginetypes.Image, err error) {
	log.Warnf(ctx, "ImageList does not implement")
	return
}

// ImageRemove removes a specific image.
func (v *Virt) ImageRemove(ctx context.Context, image string, force, prune bool) (names []string, err error) {
	log.Warnf(ctx, "ImageRemove does not implement")
	return
}

// ImagesPrune prunes one.
func (v *Virt) ImagesPrune(ctx context.Context) (err error) {
	log.Warnf(ctx, "ImagesPrune does not implement")
	return
}

// ImagePull pulls an image to local virt-node.
func (v *Virt) ImagePull(ctx context.Context, ref string, all bool) (rc io.ReadCloser, err error) {
	return
}

// ImagePush pushes to central image registry.
func (v *Virt) ImagePush(ctx context.Context, ref string) (rc io.ReadCloser, err error) {
	un := strings.Split(ref, "\n")
	if len(un) != 2 {
		return nil, errors.New("ref incorrect " + ref)
	}

	user := un[0]
	imgName := un[1]
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

	un := strings.Split(refs[0], "\n")
	if len(un) != 2 {
		return "", errors.New("ref incorrect " + refs[0])
	}
	name := un[1]

	req := virttypes.CaptureGuestReq{Name: name, User: user}
	req.ID = ID

	uimg, err := v.client.CaptureGuest(ctx, req)
	if err != nil {
		return "", err
	}

	return uimg.ID, nil
}

// ImageBuildCachePrune prunes cached one.
func (v *Virt) ImageBuildCachePrune(ctx context.Context, all bool) (reclaimed uint64, err error) {
	log.Warnf(ctx, "ImageBuildCachePrune does not implement")
	return
}

// ImageLocalDigests shows local images' digests.
func (v *Virt) ImageLocalDigests(ctx context.Context, image string) (digests []string, err error) {
	log.Warnf(ctx, "ImageLocalDigests does not implement")
	return
}

// ImageRemoteDigest shows remote one's digest.
func (v *Virt) ImageRemoteDigest(ctx context.Context, image string) (digest string, err error) {
	log.Warnf(ctx, "ImageRemoteDigest does not implement")
	return
}
