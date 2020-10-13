package calcium

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/scheduler/resources"
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
			log.Errorf("[ReallocResource] Realloc failed %v", err)
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
	containerGroups := map[string]map[[3]types.ResourceRequest][]*types.Container{}
	for nodename, containers := range nodeContainersInfo {
		containerGroups[nodename] = map[[3]types.ResourceRequest][]*types.Container{}
		for _, container := range containers {
			vbs, err := types.MergeVolumeBindings(container.Volumes, opts.Volumes)
			if err != nil {
				ch <- &types.ReallocResourceMessage{Error: err}
				return
			}
			var autoVbs types.VolumeBindings
			autoVbs, hardVbsMap[container.ID] = vbs.Divide()

			reqs := [3]types.ResourceRequest{
				resources.CPUMemResourceRequest{
					CPUQuota:        container.Quota + opts.CPU,
					CPUBind:         types.ParseTriOption(opts.BindCPU, len(container.CPU) > 0),
					Memory:          container.Memory + opts.Memory,
					MemorySoftLimit: types.ParseTriOption(opts.MemoryLimit, container.SoftLimit),
				},
				resources.NewVolumeResourceRequest(autoVbs),
				resources.StorageResourceRequest{
					Quota: container.Storage + opts.Storage,
				},
			}

			for _, req := range reqs {
				if err = req.DeployValidate(); err != nil {
					ch <- &types.ReallocResourceMessage{Error: err}
					return
				}
			}
			containerGroups[nodename][reqs] = append(containerGroups[nodename][reqs], container)
		}
	}

	for nodename, containerByReq := range containerGroups {
		for reqs, containers := range containerByReq {
			if err := c.doReallocContainersOnNode(ctx, ch, nodename, containers, []types.ResourceRequest{reqs[0], reqs[1], reqs[2]}, hardVbsMap); err != nil {

				ch <- &types.ReallocResourceMessage{Error: err}
			}
		}
	}
}

// transaction: node meta
func (c *Calcium) doReallocContainersOnNode(ctx context.Context, ch chan *types.ReallocResourceMessage, nodename string, containers []*types.Container, newReqs []types.ResourceRequest, hardVbsMap map[string]types.VolumeBindings) (err error) {
	{
		return c.withNodeLocked(ctx, nodename, func(node *types.Node) error {

			for _, container := range containers {
				recycleResources(node, container)
			}
			planMap, total, _, err := scheduler.SelectNodes(newReqs, map[string]*types.Node{node.Name: node})
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
					return c.store.UpdateNodes(ctx, node)
				},

				// then: update instances' resources
				func(ctx context.Context) error {
					for _, container := range containers {
						originalContainers = append(originalContainers, *container)
					}
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
func (c *Calcium) doUpdateResourceOnInstances(ctx context.Context, ch chan *types.ReallocResourceMessage, node *types.Node, planMap map[types.ResourceType]types.ResourcePlans, containers []*types.Container, hardVbsMap map[string]types.VolumeBindings) (rollbacks []int, err error) {
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

			resources := &types.Resources{}
			for _, plan := range planMap {
				if e = plan.Dispense(types.DispenseOptions{
					Node:               node,
					Index:              idx,
					ExistingInstances:  containers,
					HardVolumeBindings: hardVbsMap[container.ID],
				}, resources); e != nil {
					return
				}
			}

			e = c.doUpdateResourceOnInstance(ctx, node, container, *resources)
		}(container, idx)
	}

	wg.Wait()
	return rollbacks, err
}

// transaction: container meta
func (c *Calcium) doUpdateResourceOnInstance(ctx context.Context, node *types.Node, container *types.Container, resources types.Resources) error {
	originContainer := *container
	return utils.Txn(
		ctx,

		// if: update container meta
		func(ctx context.Context) error {
			container.CPU = resources.CPU
			container.Quota = resources.Quota
			container.Memory = resources.Memory
			container.SoftLimit = resources.SoftLimit
			container.Volumes = resources.Volume
			container.VolumePlan = resources.VolumePlan
			container.Storage = resources.Storage
			if !resources.CPUBind {
				container.CPU = nil
			}
			return errors.WithStack(c.store.UpdateContainer(ctx, container))
		},

		// then: update container resources
		func(ctx context.Context) error {
			r := &enginetypes.VirtualizationResource{
				CPU:           resources.CPU,
				Quota:         resources.Quota,
				NUMANode:      resources.NUMANode,
				Memory:        resources.Memory,
				SoftLimit:     resources.SoftLimit,
				Volumes:       resources.Volume.ToStringSlice(false, false),
				VolumePlan:    resources.VolumePlan.ToLiteral(),
				VolumeChanged: resources.VolumeChanged,
				Storage:       resources.Storage,
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

func recycleResources(node *types.Node, container *types.Container) {
	node.CPU.Add(container.CPU)
	node.SetCPUUsed(container.Quota, types.DecrUsage)
	node.Volume.Add(container.VolumePlan.IntoVolumeMap())
	node.SetVolumeUsed(container.VolumePlan.IntoVolumeMap().Total(), types.DecrUsage)
	node.StorageCap += container.Storage
	node.MemCap += container.Memory
	if nodeID := node.GetNUMANode(container.CPU); nodeID != "" {
		node.IncrNUMANodeMemory(nodeID, container.Memory)
	}
}

func preserveResources(node *types.Node, container *types.Container) {
	node.CPU.Sub(container.CPU)
	node.SetCPUUsed(container.Quota, types.IncrUsage)
	node.Volume.Sub(container.VolumePlan.IntoVolumeMap())
	node.SetVolumeUsed(container.VolumePlan.IntoVolumeMap().Total(), types.IncrUsage)
	node.StorageCap -= container.Storage
	node.MemCap -= container.Memory
	if nodeID := node.GetNUMANode(container.CPU); nodeID != "" {
		node.DecrNUMANodeMemory(nodeID, container.Memory)
	}
}
