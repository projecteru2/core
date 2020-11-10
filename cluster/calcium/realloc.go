package calcium

import (
	"context"

	"github.com/pkg/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

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
		if total != 1 {
			return errors.WithStack(types.ErrInsufficientRes)
		}

		originalContainer := *container
		return utils.Txn(
			ctx,

			// if update workload resources
			func(ctx context.Context) (err error) {
				return c.doReallocContainersOnInstance(ctx, node, plans, container)
			},
			// then commit changes
			func(ctx context.Context) error {
				for _, plan := range plans {
					plan.ApplyChangesOnNode(node, 1)
				}
				return c.store.UpdateNodes(ctx, node)
			},
			// rollback to origin
			func(ctx context.Context, failureByCond bool) error {
				if failureByCond {
					return nil
				}
				for _, plan := range plans {
					plan.RollbackChangesOnNode(node, 1)
				}
				node.PreserveResources(&originalContainer.ResourceMeta)
				return c.store.UpdateNodes(ctx, node)
			},

			c.config.GlobalTimeout,
		)
	})
}

func (c *Calcium) doReallocContainersOnInstance(ctx context.Context, node *types.Node, plans []resourcetypes.ResourcePlans, container *types.Container) (err error) {
	r := &types.ResourceMeta{}
	for _, plan := range plans {
		// TODO@zc: single existing instance
		// TODO@zc: no HardVolumeBindings
		if r, err = plan.Dispense(resourcetypes.DispenseOptions{
			Node:              node,
			Index:             1,
			ExistingInstances: []*types.Container{container},
		}, r); err != nil {
			return
		}
	}

	originalContainer := *container
	return utils.Txn(
		ctx,

		// if: update container resources
		func(ctx context.Context) error {
			r := &enginetypes.VirtualizationResource{
				CPU:           r.CPU,
				Quota:         r.CPUQuotaLimit,
				NUMANode:      r.NUMANode,
				Memory:        r.MemoryLimit,
				Volumes:       r.VolumeLimit.ToStringSlice(false, false),
				VolumePlan:    r.VolumePlanLimit.ToLiteral(),
				VolumeChanged: r.VolumeChanged,
				Storage:       r.StorageLimit,
			}
			return errors.WithStack(node.Engine.VirtualizationUpdateResource(ctx, container.ID, r))
		},

		// then: update container meta
		func(ctx context.Context) error {
			container.CPUQuotaRequest = r.CPUQuotaRequest
			container.CPUQuotaLimit = r.CPUQuotaLimit
			container.CPU = r.CPU
			container.MemoryRequest = r.MemoryRequest
			container.MemoryLimit = r.MemoryLimit
			container.VolumeRequest = r.VolumeRequest
			container.VolumePlanRequest = r.VolumePlanRequest
			container.VolumeLimit = r.VolumeLimit
			container.VolumePlanLimit = r.VolumePlanLimit
			container.StorageRequest = r.StorageRequest
			container.StorageLimit = r.StorageLimit
			return errors.WithStack(c.store.UpdateContainer(ctx, container))
		},

		// rollback: container meta
		func(ctx context.Context, failureByCond bool) error {
			if failureByCond {
				return nil
			}
			return errors.WithStack(c.store.UpdateContainer(ctx, &originalContainer))
		},

		c.config.GlobalTimeout,
	)
}
