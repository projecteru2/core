package calcium

import (
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"golang.org/x/net/context"
)

func distributionInspect(ctx context.Context, node *types.Node, image string, digests []string) bool {
	logger := log.WithFunc("calcium.distributionInspect")
	remoteDigest, err := node.Engine.ImageRemoteDigest(ctx, image)
	if err != nil {
		logger.Error(ctx, err, "get manifest failed")
		return false
	}

	for _, digest := range digests {
		if digest == remoteDigest {
			logger.Debugf(ctx, "local digest %s", digest)
			logger.Debugf(ctx, "remote digest %s", remoteDigest)
			return true
		}
	}
	return false
}

// Pull an image
func pullImage(ctx context.Context, node *types.Node, image string) error {
	logger := log.WithFunc("calcium.pullImage").WithField("image", image)
	logger.Info(ctx, "Pulling image")
	if image == "" {
		return types.ErrNoImage
	}

	// check local
	exists := false
	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	if err != nil {
		logger.Errorf(ctx, err, "Check image failed %+v", err)
	} else {
		logger.Debug(ctx, "Local Image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, digests) {
		logger.Debug(ctx, "Image cached, skip pulling")
		return nil
	}

	logger.Info(ctx, "Image not cached, pulling")
	rc, err := node.Engine.ImagePull(ctx, image, false)
	defer utils.EnsureReaderClosed(ctx, rc)
	if err != nil {
		logger.Errorf(ctx, err, "Error during pulling image %s", image)
		return err
	}
	logger.Infof(ctx, "Done pulling image %s", image)
	return nil
}
