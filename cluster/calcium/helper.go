package calcium

import (
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func distributionInspect(ctx context.Context, node *types.Node, image string, digests []string) bool {
	remoteDigest, err := node.Engine.ImageRemoteDigest(ctx, image)
	if err != nil {
		log.Errorf(ctx, "[distributionInspect] get manifest failed %v", err)
		return false
	}

	for _, digest := range digests {
		if digest == remoteDigest {
			log.Debugf(ctx, "[distributionInspect] Local digest %s", digest)
			log.Debugf(ctx, "[distributionInspect] Remote digest %s", remoteDigest)
			return true
		}
	}
	return false
}

// Pull an image
func pullImage(ctx context.Context, node *types.Node, image string) error {
	log.Infof(ctx, "[pullImage] Pulling image %s", image)
	if image == "" {
		return errors.WithStack(types.ErrNoImage)
	}

	// check local
	exists := false
	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	if err != nil {
		log.Errorf(ctx, "[pullImage] Check image failed %v", err)
	} else {
		log.Debug(ctx, "[pullImage] Local Image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, digests) {
		log.Debug(ctx, "[pullImage] Image cached, skip pulling")
		return nil
	}

	log.Info("[pullImage] Image not cached, pulling")
	rc, err := node.Engine.ImagePull(ctx, image, false)
	defer utils.EnsureReaderClosed(ctx, rc)
	if err != nil {
		log.Errorf(ctx, "[pullImage] Error during pulling image %s: %v", image, err)
		return errors.WithStack(err)
	}
	log.Infof(ctx, "[pullImage] Done pulling image %s", image)
	return nil
}
