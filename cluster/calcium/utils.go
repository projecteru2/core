package calcium

import (
	"context"
	"slices"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) withResourceReleased(ctx context.Context, node *types.Node, workload *types.Workload, then func(context.Context) error) error {
	_, err := utils.Txn(
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
	return err
}

func (c *Calcium) invokePoolAsync(f func()) {
	utils.SentryGo(func() { _ = c.pool.Invoke(f) })
}

func perNode[T any](c *Calcium, nodes []*types.Node, work func(*types.Node, chan<- T)) chan T {
	ch := make(chan T)
	utils.SentryGo(func() {
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

func removeWorkloadByName(ctx context.Context, node *types.Node, name string) error {
	info, err := node.Engine.VirtualizationInspect(ctx, name)
	if err != nil {
		if errors.Is(err, types.ErrWorkloadNotExists) {
			return nil
		}
		return err
	}

	if err = node.Engine.VirtualizationRemove(ctx, info.ID, true, true); err != nil && !errors.Is(err, types.ErrWorkloadNotExists) {
		return err
	}
	return nil
}

func inspectDistribution(ctx context.Context, node *types.Node, image string, digests []string) bool {
	logger := log.WithFunc("calcium.inspectDistribution")
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
	case inspectDistribution(ctx, node, image, digests):
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

func send[T any](ctx context.Context, ch chan<- T, msg T) error {
	select {
	case ch <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
