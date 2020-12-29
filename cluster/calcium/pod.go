package calcium

import (
	"context"

	"github.com/projecteru2/core/types"
)

// AddPod add pod
func (c *Calcium) AddPod(ctx context.Context, podname, desc string) (*types.Pod, error) {
	if podname == "" {
		return nil, types.ErrEmptyPodName
	}
	return c.store.AddPod(ctx, podname, desc)
}

// RemovePod remove pod
func (c *Calcium) RemovePod(ctx context.Context, podname string) error {
	if podname == "" {
		return types.ErrEmptyPodName
	}
	return c.withNodesLocked(ctx, podname, []string{}, nil, true, func(ctx context.Context, nodes map[string]*types.Node) error {
		// TODO dissociate workload to node
		// TODO should remove node first
		return c.store.RemovePod(ctx, podname)
	})
}

// GetPod get one pod
func (c *Calcium) GetPod(ctx context.Context, podname string) (*types.Pod, error) {
	if podname == "" {
		return nil, types.ErrEmptyPodName
	}
	return c.store.GetPod(ctx, podname)
}

// ListPods show pods
func (c *Calcium) ListPods(ctx context.Context) ([]*types.Pod, error) {
	return c.store.GetAllPods(ctx)
}
