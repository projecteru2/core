package calcium

import (
	"context"
	"slices"
	"sync"

	"github.com/cockroachdb/errors"

	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// releaseWorkers bounds the engine removes in flight on one node.
const releaseWorkers = 16

// withResourceReleased journals the node, runs the removal, then gives the workload's usage back under the node lock; the usage stays charged until the workload is gone, and a failed removal or release stays in the journal for repair.
func (c *Calcium) withResourceReleased(ctx context.Context, logger *log.Fields, node *types.Node, workload *types.Workload, remove func(context.Context) error) error {
	nodeCommit, err := c.journal(ctx, logger, eventWorkloadResourceAllocated, []*types.Node{node})
	if err != nil {
		return err
	}
	if err = remove(ctx); err != nil {
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

// releaseWorkloads runs release under each workload's lock, node by node with releaseWorkers in flight per node, gives the usage back, reports each outcome, remaps the node afterwards and calls done once every node is through.
func (c *Calcium) releaseWorkloads(ctx context.Context, logger *log.Fields, IDs []string, release func(context.Context, *types.Node, *types.Workload) error, report func(workloadID string, err error) error, done func()) error {
	nodeWorkloadGroup, err := c.groupWorkloadsByNode(ctx, IDs)
	if err != nil {
		return err
	}
	utils.SentryGo(func() {
		defer done()
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for nodename, workloadIDs := range nodeWorkloadGroup {
			wg.Add(1)
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				node, err := c.store.GetNode(ctx, nodename)
				if err != nil {
					logger.WithField("node", nodename).Error(ctx, err, "failed to get node")
					for _, workloadID := range workloadIDs {
						_ = report(workloadID, err)
					}
					return
				}
				var releases errgroup.Group
				releases.SetLimit(releaseWorkers)
				for _, workloadID := range workloadIDs {
					releases.Go(func() error {
						defer log.SentryDefer()
						err := c.withWorkloadLocked(ctx, workloadID, false, func(ctx context.Context, workload *types.Workload) error {
							return c.withResourceReleased(ctx, logger, node, workload, func(ctx context.Context) error { return release(ctx, node, workload) })
						})
						return report(workloadID, err)
					})
				}
				_ = releases.Wait()
				c.invokePoolAsync(func() { c.RemapResourceAndLog(ctx, logger, node.Name) })
			})
		}
	})
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
