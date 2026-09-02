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

// withResourceReleased journals the node, runs the removal, then gives the workload's usage back under the node lock; the usage stays charged until the workload is gone, and a failed release stays in the journal for repair.
func (c *Calcium) withResourceReleased(ctx context.Context, logger *log.Fields, node *types.Node, workload *types.Workload, remove func(context.Context) error) error {
	nodeCommit, err := c.journal(ctx, logger, eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	if err = remove(ctx); err != nil {
		nodeCommit()
		return err
	}
	err = c.withNodeKeyLocked(ctx, node, func(ctx context.Context) (err error) {
		_, _, err = c.rmgr.SetNodeResourceUsage(ctx, node.Name, nil, nil, []resourcetypes.Resources{workload.Resources}, true, plugins.Decr)
		return err
	})
	if err != nil {
		logger.WithField("id", workload.ID).Error(ctx, err, "usage release left to the journal")
		return nil
	}
	nodeCommit()
	return nil
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
