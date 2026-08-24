package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
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

func pullImage(ctx context.Context, node *types.Node, image string) error {
	logger := log.WithFunc("calcium.pullImage").WithField("node", node.Name).WithField("image", image)
	logger.Info(ctx, "pulling image")
	if image == "" {
		return types.ErrNoImage
	}

	exists := false
	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	if err != nil {
		logger.Error(ctx, err, "check image failed")
	} else {
		logger.Debug(ctx, "local image exists")
		exists = true
	}

	if exists && distributionInspect(ctx, node, image, digests) {
		logger.Debug(ctx, "image cached, skip pulling")
		return nil
	}

	logger.Info(ctx, "image not cached, pulling")
	rc, err := node.Engine.ImagePull(ctx, image, false)
	defer utils.EnsureReaderClosed(ctx, rc)
	if err != nil {
		logger.Errorf(ctx, err, "failed to pull image %s", image)
		return err
	}
	logger.Infof(ctx, "done pulling image %s", image)
	return nil
}
