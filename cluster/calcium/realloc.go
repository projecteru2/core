package calcium

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// nodename -> container list
type nodeContainers map[string][]*types.Container

// ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, opts *types.ReallocOptions) (chan *types.ReallocResourceMessage, error) {
	ch := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch)
		if err := c.withContainersLocked(ctx, opts.IDs, func(containers map[string]*types.Container) error {
			// Pod-Node-Containers
			containersInfo := map[*types.Pod]nodeContainers{}
			// Pod cache
			podCache := map[string]*types.Pod{}
			var err error
			for _, container := range containers {
				pod, ok := podCache[container.Podname]
				if !ok {
					pod, err = c.store.GetPod(ctx, container.Podname)
					if err != nil {
						ch <- &types.ReallocResourceMessage{
							ContainerID: container.ID,
							Error:       err,
						}
						continue
					}
					podCache[container.Podname] = pod
					containersInfo[pod] = nodeContainers{}
				}
				if _, ok = containersInfo[pod][container.Nodename]; !ok {
					containersInfo[pod][container.Nodename] = []*types.Container{}
				}
				containersInfo[pod][container.Nodename] = append(containersInfo[pod][container.Nodename], container)
			}

			wg := sync.WaitGroup{}
			wg.Add(len(containersInfo))
			// deal with normal container
			for _, nodeContainersInfo := range containersInfo {
				go func(nodeContainersInfo nodeContainers) {
					defer wg.Done()
					c.doReallocContainersOnPod(ctx, ch, nodeContainersInfo, opts)
				}(nodeContainersInfo)
			}
			wg.Wait()
			return nil
		}); err != nil {
			log.Errorf("[ReallocResource] Realloc failed %+v", err)
			for _, ID := range opts.IDs {
				ch <- &types.ReallocResourceMessage{
					ContainerID: ID,
					Error:       err,
				}
			}
		}
	}()
	return ch, nil
}

// group containers by node and requests
func (c *Calcium) doReallocContainersOnPod(ctx context.Context, ch chan *types.ReallocResourceMessage, nodeContainersInfo nodeContainers, opts *types.ReallocOptions) {
	hardVbsMap := map[string]types.VolumeBindings{}
	containerGroups := map[string]map[resourcetypes.ResourceRequirements][]*types.Container{}
	for nodename, containers := range nodeContainersInfo {
		containerGroups[nodename] = map[resourcetypes.ResourceRequirements][]*types.Container{}
		for _, container := range containers {
			if err := func() (err error) {
				var (
					autoVbsRequest, autoVbsLimit types.VolumeBindings
					rrs                          resourcetypes.ResourceRequirements
				)
				autoVbsRequest, hardVbsMap[container.ID] = types.MergeVolumeBindings(container.VolumeRequest, opts.VolumeRequest, opts.Volumes).Divide()
				autoVbsLimit, _ = types.MergeVolumeBindings(container.VolumeLimit, opts.VolumeLimit, opts.Volumes).Divide()

				rrs, err = resources.NewResourceRequirements(
					types.Resource{
						CPUQuotaRequest: container.CPUQuotaRequest + opts.CPURequest + opts.CPU,
						CPULimit:        container.QuotaLimit + opts.CPULimit + opts.CPU,
						CPUBind:         types.ParseTriOption(opts.BindCPUOpt, len(container.CPURequest) > 0),
						MemoryRequest:   container.MemoryRequest + opts.MemoryRequest + opts.Memory,
						MemoryLimit:     container.MemoryLimit + opts.MemoryLimit + opts.Memory,
						MemorySoft:      types.ParseTriOption(opts.MemoryLimitOpt, container.SoftLimit),
						VolumeRequest:   autoVbsRequest,
						VolumeLimit:     autoVbsLimit,
						StorageRequest:  container.StorageRequest + opts.StorageRequest + opts.Storage,
						StorageLimit:    container.StorageLimit + opts.StorageLimit + opts.Storage,
					})

				containerGroups[nodename][rrs] = append(containerGroups[nodename][rrs], container)
				return

			}(); err != nil {
				log.Errorf("[ReallocResource.doReallocContainersOnPod] Realloc failed: %+v", err)
				ch <- &types.ReallocResourceMessage{Error: err}
				return
			}
		}
	}

	for nodename, containerByApps := range containerGroups {
		for rrs, containers := range containerByApps {
			if err := c.doReallocContainersOnNode(ctx, ch, nodename, containers, rrs, hardVbsMap); err != nil {

				log.Errorf("[ReallocResource.doReallocContainersOnPod] Realloc failed: %+v", err)
				ch <- &types.ReallocResourceMessage{Error: err}
			}
		}
	}
}

// transaction: node meta
func (c *Calcium) doReallocContainersOnNode(ctx context.Context, ch chan *types.ReallocResourceMessage, nodename string, containers []*types.Container, rrs resourcetypes.ResourceRequirements, hardVbsMap map[string]types.VolumeBindings) (err error) {
	{
		return c.withNodeLocked(ctx, nodename, func(node *types.Node) error {

			for _, container := range containers {
				recycleResources(node, container)
			}
			planMap, total, _, err := resources.SelectNodes(rrs, map[string]*types.Node{node.Name: node})
			if err != nil {
				return errors.WithStack(err)
			}
			if total < len(containers) {
				return errors.WithStack(types.ErrInsufficientRes)
			}

			var (
				rollbacks          []int
				originalContainers []types.Container
			)

			return utils.Txn(
				ctx,

				// if: commit changes of realloc resources
				func(ctx context.Context) (err error) {
					for _, plan := range planMap {
						plan.ApplyChangesOnNode(node, utils.Range(len(containers))...)
					}
					rollbacks = utils.Range(len(containers))
					for _, container := range containers {
						originalContainers = append(originalContainers, *container)
					}
					return c.store.UpdateNodes(ctx, node)
				},

				// then: update instances' resources
				func(ctx context.Context) error {
					rollbacks, err = c.doUpdateResourceOnInstances(ctx, ch, node, planMap, containers, hardVbsMap)
					return err
				},

				// rollback: back to origin
				func(ctx context.Context) error {
					for _, plan := range planMap {
						plan.RollbackChangesOnNode(node, rollbacks...)
					}
					for _, idx := range rollbacks {
						preserveResources(node, &originalContainers[idx])
					}
					return c.store.UpdateNodes(ctx, node)
				},
				c.config.GlobalTimeout,
			)
		})
	}
}

// boundary: chan *types.ReallocResourceMessage
func (c *Calcium) doUpdateResourceOnInstances(ctx context.Context, ch chan *types.ReallocResourceMessage, node *types.Node, planMap map[types.ResourceType]resourcetypes.ResourcePlans, containers []*types.Container, hardVbsMap map[string]types.VolumeBindings) (rollbacks []int, err error) {
	wg := sync.WaitGroup{}
	wg.Add(len(containers))

	for idx, container := range containers {
		go func(container *types.Container, idx int) {
			var e error
			msg := &types.ReallocResourceMessage{ContainerID: container.ID}
			defer func() {
				if e != nil {
					err = e
					msg.Error = e
					rollbacks = append(rollbacks, idx)
				}
				ch <- msg
				wg.Done()
			}()

			rsc := &types.Resources{}
			for _, plan := range planMap {
				if e = plan.Dispense(resourcetypes.DispenseOptions{
					Node:               node,
					Index:              idx,
					ExistingInstances:  containers,
					HardVolumeBindings: hardVbsMap[container.ID],
				}, rsc); e != nil {
					return
				}
			}

			e = c.doUpdateResourceOnInstance(ctx, node, container, *rsc)
		}(container, idx)
	}

	wg.Wait()
	return rollbacks, errors.WithStack(err)
}

// transaction: container meta
func (c *Calcium) doUpdateResourceOnInstance(ctx context.Context, node *types.Node, container *types.Container, rsc types.Resources) error {
	originContainer := *container
	return utils.Txn(
		ctx,

		// if: update container meta
		func(ctx context.Context) error {
			container.CPURequest = rsc.CPURequest
			container.QuotaRequest = rsc.CPUQuotaRequest
			container.QuotaLimit = rsc.CPUQuotaLimit
			container.MemoryRequest = rsc.MemoryRequest
			container.MemoryLimit = rsc.MemoryLimit
			container.SoftLimit = rsc.MemorySoftLimit
			container.VolumeRequest = rsc.VolumeRequest
			container.VolumePlanRequest = rsc.VolumePlanRequest
			container.VolumeLimit = rsc.VolumeLimit
			container.VolumePlanLimit = rsc.VolumePlanLimit
			container.StorageRequest = rsc.StorageRequest
			container.StorageLimit = rsc.StorageLimit
			return errors.WithStack(c.store.UpdateContainer(ctx, container))
		},

		// then: update container resources
		func(ctx context.Context) error {
			r := &enginetypes.VirtualizationResource{
				CPU:           rsc.CPURequest,
				Quota:         rsc.CPUQuotaLimit,
				NUMANode:      rsc.NUMANode,
				Memory:        rsc.MemoryLimit,
				SoftLimit:     rsc.MemorySoftLimit,
				Volumes:       rsc.VolumeLimit.ToStringSlice(false, false),
				VolumePlan:    rsc.VolumePlanLimit.ToLiteral(),
				VolumeChanged: rsc.VolumeChanged,
				Storage:       rsc.StorageLimit,
			}
			return errors.WithStack(node.Engine.VirtualizationUpdateResource(ctx, container.ID, r))
		},

		// rollback: container meta
		func(ctx context.Context) error {
			return errors.WithStack(c.store.UpdateContainer(ctx, &originContainer))
		},

		c.config.GlobalTimeout,
	)
}
