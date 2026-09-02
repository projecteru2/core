package calcium

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) AddPod(ctx context.Context, podname, desc string) (*types.Pod, error) {
	logger := log.WithFunc("calcium.AddPod").WithField("podname", podname)
	if podname == "" {
		logger.Error(ctx, types.ErrEmptyPodName)
		return nil, types.ErrEmptyPodName
	}
	pod, err := c.store.AddPod(ctx, podname, desc)
	logger.Error(ctx, err)
	return pod, err
}

func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	logger := log.WithFunc("calcium.RemovePod").WithField("podname", podname)
	if podname == "" {
		logger.Error(ctx, types.ErrEmptyPodName)
		return types.ErrEmptyPodName
	}

	return c.withPodLocked(ctx, podname, func(ctx context.Context, _ map[string]*types.Node) error {
		err := c.store.RemovePod(ctx, podname)
		logger.Error(ctx, err)
		return err
	})
}

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

func (c *Calcium) ListPods(ctx context.Context) ([]*types.Pod, error) {
	pods, err := c.store.GetAllPods(ctx)
	log.WithFunc("calcium.ListPods").Error(ctx, err)
	return pods, err
}
