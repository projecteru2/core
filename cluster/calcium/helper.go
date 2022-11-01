package calcium

import (
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"golang.org/x/net/context"
)

func distributionInspect(ctx context.Context, node *types.Node, image string, digests []string) bool {
	remoteDigest, err := node.Engine.ImageRemoteDigest(ctx, image)
	if err != nil {
		log.Error(ctx, err, "[distributionInspect] get manifest failed")
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
		return types.ErrNoImage
	}

	// check local
	exists := false
	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	if err != nil {
		log.Errorf(ctx, err, "[pullImage] Check image failed %+v", err)
	} else {
		log.Debug(ctx, "[pullImage] Local Image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, digests) {
		log.Debug(ctx, "[pullImage] Image cached, skip pulling")
		return nil
	}

	log.Info(ctx, "[pullImage] Image not cached, pulling")
	rc, err := node.Engine.ImagePull(ctx, image, false)
	defer utils.EnsureReaderClosed(ctx, rc)
	if err != nil {
		log.Errorf(ctx, err, "[pullImage] Error during pulling image %s", image)
		return err
	}
	log.Infof(ctx, "[pullImage] Done pulling image %s", image)
	return nil
}
