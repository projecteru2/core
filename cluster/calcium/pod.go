package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"

	"github.com/pkg/errors"
)

// AddPod add pod
func (c *Calcium) AddPod(ctx context.Context, podname, desc string) (*types.Pod, error) {
	logger := log.WithField("Calcium", "AddPod").WithField("podname", podname).WithField("desc", desc)
	if podname == "" {
		return nil, logger.ErrWithTracing(ctx, errors.WithStack(types.ErrEmptyPodName))
	}
	pod, err := c.store.AddPod(ctx, podname, desc)
	return pod, logger.ErrWithTracing(ctx, errors.WithStack(err))
}

// RemovePod remove pod
func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	logger := log.WithField("Calcium", "RemovePod").WithField("podname", podname)
	if podname == "" {
		return logger.ErrWithTracing(ctx, errors.WithStack(types.ErrEmptyPodName))
	}

	nf := types.NodeFilter{
		Podname: podname,
		All:     true,
	}
	return c.withNodesPodLocked(ctx, nf, func(ctx context.Context, nodes map[string]*types.Node) error {
		// TODO dissociate workload to node
		// TODO should remove node first
		return logger.ErrWithTracing(ctx, errors.WithStack(c.store.RemovePod(ctx, podname)))
	})
}

// GetPod get one pod
func (c *Calcium) GetPod(ctx context.Context, podname string) (*types.Pod, error) {
	logger := log.WithField("Calcium", "GetPod").WithField("podname", podname)
	if podname == "" {
		return nil, logger.ErrWithTracing(ctx, errors.WithStack(types.ErrEmptyPodName))
	}
	pod, err := c.store.GetPod(ctx, podname)
	return pod, logger.ErrWithTracing(ctx, errors.WithStack(err))
}

// ListPods show pods
func (c *Calcium) ListPods(ctx context.Context) ([]*types.Pod, error) {
	pods, err := c.store.GetAllPods(ctx)
	return pods, log.WithField("Calcium", "ListPods").ErrWithTracing(ctx, errors.WithStack(err))
}
