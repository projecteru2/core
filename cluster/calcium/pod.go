package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// AddPod add pod
func (c *Calcium) AddPod(ctx context.Context, podname, desc string) (*types.Pod, error) {
	logger := log.WithFunc("calcium.AddPod").WithField("podname", podname).WithField("desc", desc)
	if podname == "" {
		logger.Error(ctx, types.ErrEmptyPodName)
		return nil, types.ErrEmptyPodName
	}
	pod, err := c.store.AddPod(ctx, podname, desc)
	logger.Error(ctx, err)
	return pod, err
}

// RemovePod remove pod
func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	logger := log.WithFunc("calcium.RemovePod").WithField("podname", podname)
	if podname == "" {
		logger.Error(ctx, types.ErrEmptyPodName)
		return types.ErrEmptyPodName
	}

	nodeFilter := &types.NodeFilter{
		Podname: podname,
		All:     true,
	}
	return c.withNodesPodLocked(ctx, nodeFilter, func(ctx context.Context, nodes map[string]*types.Node) error {
		// TODO dissociate workload to node
		// TODO should remove node first
		err := c.store.RemovePod(ctx, podname)
		logger.Error(ctx, err)
		return err
	})
}

// GetPod get one pod
func (c *Calcium) GetPod(ctx context.Context, podname string) (*types.Pod, error) {
	logger := log.WithFunc("calcium.GetPod").WithField("podname", podname)
	if podname == "" {
		logger.Error(ctx, types.ErrEmptyPodName)
		return nil, types.ErrEmptyPodName
	}
	pod, err := c.store.GetPod(ctx, podname)
	logger.Error(ctx, err)
	return pod, err
}

// ListPods show pods
func (c *Calcium) ListPods(ctx context.Context) ([]*types.Pod, error) {
	pods, err := c.store.GetAllPods(ctx)
	log.WithFunc("calcium.ListPods").Error(ctx, err)
	return pods, err
}
