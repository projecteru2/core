package calcium

import (
	"context"
	"slices"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) withResourceReleased(ctx context.Context, node *types.Node, workload *types.Workload, then func(context.Context) error) error {
	return utils.Txn(
		ctx,
		func(ctx context.Context) (err error) {
			_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []resourcetypes.Resources{workload.Resources}, true, plugins.Decr)
			return err
		},
		then,
		func(ctx context.Context, failedByCond bool) (err error) {
			if failedByCond {
				return nil
			}
			_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []resourcetypes.Resources{workload.Resources}, true, plugins.Incr)
			return err
		},
		c.config.GlobalTimeout,
	)
}

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

	if slices.Contains(digests, remoteDigest) {
		logger.Debugf(ctx, "digest matched %s", remoteDigest)
		return true
	}
	return false
}

func pullImage(ctx context.Context, node *types.Node, image string) error {
	logger := log.WithFunc("calcium.pullImage").WithField("node", node.Name).WithField("image", image)
	if image == "" {
		return types.ErrNoImage
	}

	digests, err := node.Engine.ImageLocalDigests(ctx, image)
	switch {
	case err != nil:
		logger.Warnf(ctx, "check image failed: %+v", err)
	case distributionInspect(ctx, node, image, digests):
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
