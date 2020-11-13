package calcium

import (
	"context"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// ReallocResource updates workload resource dynamically
func (c *Calcium) ReallocResource(ctx context.Context, opts *types.ReallocOptions) (err error) {
	return c.withContainerLocked(ctx, opts.ID, func(container *types.Container) error {
		rrs, err := resources.MakeRequests(
			types.ResourceOptions{
				CPUQuotaRequest: container.CPUQuotaRequest + opts.ResourceOpts.CPUQuotaRequest,
				CPUQuotaLimit:   container.CPUQuotaLimit + opts.ResourceOpts.CPUQuotaLimit,
				CPUBind:         types.ParseTriOption(opts.CPUBindOpts, len(container.CPU) > 0),
				MemoryRequest:   container.MemoryRequest + opts.ResourceOpts.MemoryRequest,
				MemoryLimit:     container.MemoryLimit + opts.ResourceOpts.MemoryLimit,
				StorageRequest:  container.StorageRequest + opts.ResourceOpts.StorageRequest,
				StorageLimit:    container.StorageLimit + opts.ResourceOpts.StorageLimit,
				VolumeRequest:   types.MergeVolumeBindings(container.VolumeRequest, opts.ResourceOpts.VolumeRequest),
				VolumeLimit:     types.MergeVolumeBindings(container.VolumeLimit, opts.ResourceOpts.VolumeLimit),
			},
		)
		if err != nil {
			return errors.WithStack(err)
		}
		return c.doReallocOnNode(ctx, container.Nodename, container, rrs)
	})
}

// transaction: node resource
func (c *Calcium) doReallocOnNode(ctx context.Context, nodename string, container *types.Container, rrs resourcetypes.ResourceRequests) error {
	return c.withNodeLocked(ctx, nodename, func(node *types.Node) error {
		node.RecycleResources(&container.ResourceMeta)
		_, total, plans, err := resources.SelectNodesByResourceRequests(rrs, map[string]*types.Node{node.Name: node})
		if err != nil {
			return errors.WithStack(err)
		}
		if total < 1 {
			return errors.WithStack(types.ErrInsufficientRes)
		}

		originalContainer := *container
		resourceMeta := &types.ResourceMeta{}
		return utils.Txn(
			ctx,

			// if update workload resources
			func(ctx context.Context) (err error) {
				resourceMeta := &types.ResourceMeta{}
				for _, plan := range plans {
					if resourceMeta, err = plan.Dispense(resourcetypes.DispenseOptions{
						Node:             node,
						ExistingInstance: container,
					}, resourceMeta); err != nil {
						return
					}
				}
				return errors.WithStack(c.doReallocContainersOnInstance(ctx, node.Engine, resourceMeta, container))
			},
			// then commit changes
			func(ctx context.Context) error {
				for _, plan := range plans {
					plan.ApplyChangesOnNode(node, 1)
				}
				return errors.WithStack(c.store.UpdateNodes(ctx, node))
			},
			// no need rollback
			func(ctx context.Context, failureByCond bool) (err error) {
				if failureByCond {
					return
				}
				r := &types.ResourceMeta{
					CPUQuotaRequest:   originalContainer.CPUQuotaRequest,
					CPUQuotaLimit:     originalContainer.CPUQuotaLimit,
					CPU:               originalContainer.CPU,
					NUMANode:          originalContainer.NUMANode,
					MemoryRequest:     originalContainer.MemoryRequest,
					MemoryLimit:       originalContainer.MemoryLimit,
					VolumeRequest:     originalContainer.VolumeRequest,
					VolumeLimit:       originalContainer.VolumeLimit,
					VolumePlanRequest: originalContainer.VolumePlanRequest,
					VolumePlanLimit:   originalContainer.VolumePlanLimit,
					VolumeChanged:     resourceMeta.VolumeChanged,
					StorageRequest:    originalContainer.StorageRequest,
					StorageLimit:      originalContainer.StorageLimit,
				}
				return errors.WithStack(c.doReallocContainersOnInstance(ctx, node.Engine, r, container))
			},

			c.config.GlobalTimeout,
		)
	})
}

func (c *Calcium) doReallocContainersOnInstance(ctx context.Context, engine engine.API, resourceMeta *types.ResourceMeta, container *types.Container) (err error) {

	originalContainer := *container
	return utils.Txn(
		ctx,

		// if: update container resources
		func(ctx context.Context) error {
			r := &enginetypes.VirtualizationResource{
				CPU:           resourceMeta.CPU,
				Quota:         resourceMeta.CPUQuotaLimit,
				NUMANode:      resourceMeta.NUMANode,
				Memory:        resourceMeta.MemoryLimit,
				Volumes:       resourceMeta.VolumeLimit.ToStringSlice(false, false),
				VolumePlan:    resourceMeta.VolumePlanLimit.ToLiteral(),
				VolumeChanged: resourceMeta.VolumeChanged,
				Storage:       resourceMeta.StorageLimit,
			}
			return errors.WithStack(engine.VirtualizationUpdateResource(ctx, container.ID, r))
		},

		// then: update container meta
		func(ctx context.Context) error {
			container.CPUQuotaRequest = resourceMeta.CPUQuotaRequest
			container.CPUQuotaLimit = resourceMeta.CPUQuotaLimit
			container.CPU = resourceMeta.CPU
			container.NUMANode = resourceMeta.NUMANode
			container.MemoryRequest = resourceMeta.MemoryRequest
			container.MemoryLimit = resourceMeta.MemoryLimit
			container.VolumeRequest = resourceMeta.VolumeRequest
			container.VolumePlanRequest = resourceMeta.VolumePlanRequest
			container.VolumeLimit = resourceMeta.VolumeLimit
			container.VolumePlanLimit = resourceMeta.VolumePlanLimit
			container.StorageRequest = resourceMeta.StorageRequest
			container.StorageLimit = resourceMeta.StorageLimit
			return errors.WithStack(c.store.UpdateContainer(ctx, container))
		},

		// rollback: container meta
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			r := &enginetypes.VirtualizationResource{
				CPU:           originalContainer.CPU,
				Quota:         originalContainer.CPUQuotaLimit,
				NUMANode:      originalContainer.NUMANode,
				Memory:        originalContainer.MemoryLimit,
				Volumes:       originalContainer.VolumeLimit.ToStringSlice(false, false),
				VolumePlan:    originalContainer.VolumePlanLimit.ToLiteral(),
				VolumeChanged: resourceMeta.VolumeChanged,
				Storage:       originalContainer.StorageLimit,
			}
			return errors.WithStack(engine.VirtualizationUpdateResource(ctx, container.ID, r))
		},

		c.config.GlobalTimeout,
	)
}
