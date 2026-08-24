package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func perNode[T any](c *Calcium, nodes []*types.Node, work func(*types.Node, chan<- T)) chan T {
	ch := make(chan T)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(nodes))
		defer wg.Wait()
		for _, node := range nodes {
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				work(node, ch)
			})
		}
	})
	return ch
}

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
