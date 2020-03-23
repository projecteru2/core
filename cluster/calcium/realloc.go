package calcium

import (
	"context"
	"strconv"
	"strings"
	"sync"

	"github.com/sanity-io/litter"

	"github.com/pkg/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// nodename -> container list
type nodeContainers map[string][]*types.Container

// volume:cpu:memory
type volCPUMemNodeContainers map[string]map[float64]map[int64]nodeContainers

// ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, IDs []string, cpu float64, memory int64, volumes types.VolumeBindings, bindCPUOpt types.BindCPUOptions) (chan *types.ReallocResourceMessage, error) {
	ch := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch)
		if err := c.withContainersLocked(ctx, IDs, func(containers map[string]*types.Container) error {
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
			for pod, nodeContainersInfo := range containersInfo {
				go func(pod *types.Pod, nodeContainersInfo nodeContainers) {
					defer wg.Done()
					c.doReallocContainer(ctx, ch, pod, nodeContainersInfo, cpu, memory, volumes, bindCPUOpt)
				}(pod, nodeContainersInfo)
			}
			wg.Wait()
			return nil
		}); err != nil {
			log.Errorf("[ReallocResource] Realloc failed %v", err)
			for _, ID := range IDs {
				ch <- &types.ReallocResourceMessage{
					ContainerID: ID,
					Error:       err,
				}
			}
		}
	}()
	return ch, nil
}

func calVolCPUMemNodeContainerInfo(nodename string, container *types.Container, cpu float64, memory int64, volumes types.VolumeBindings, volCPUMemNodeContainersInfo volCPUMemNodeContainers, hardVbsForContainer map[string]types.VolumeBindings) error {
	newCPU := utils.Round(container.Quota + cpu)
	newMem := container.Memory + memory
	if newCPU < 0 || newMem < 0 {
		log.Errorf("[doReallocContainer] New resource invalid %s, cpu %f, mem %d", container.ID, newCPU, newMem)
		return types.ErrInvalidRes
	}

	autoVolumes, hardVolumes, err := container.Volumes.Merge(volumes)
	hardVbsForContainer[container.ID] = hardVolumes
	if err != nil {
		log.Errorf("[doReallocContainer] New resource invalid %s, vol %v, err %v", container.ID, volumes, err)
		return err
	}
	newAutoVol := strings.Join(autoVolumes.ToStringSlice(true, false), ",")

	if _, ok := volCPUMemNodeContainersInfo[newAutoVol]; !ok {
		volCPUMemNodeContainersInfo[newAutoVol] = map[float64]map[int64]nodeContainers{}
	}
	if _, ok := volCPUMemNodeContainersInfo[newAutoVol][newCPU]; !ok {
		volCPUMemNodeContainersInfo[newAutoVol][newCPU] = map[int64]nodeContainers{}
	}
	if _, ok := volCPUMemNodeContainersInfo[newAutoVol][newCPU][newMem]; !ok {
		volCPUMemNodeContainersInfo[newAutoVol][newCPU][newMem] = nodeContainers{}
	}
	if _, ok := volCPUMemNodeContainersInfo[newAutoVol][newCPU][newMem][nodename]; !ok {
		volCPUMemNodeContainersInfo[newAutoVol][newCPU][newMem][nodename] = []*types.Container{}
	}
	volCPUMemNodeContainersInfo[newAutoVol][newCPU][newMem][nodename] = append(volCPUMemNodeContainersInfo[newAutoVol][newCPU][newMem][nodename], container)
	return nil
}

func calVolCPUMemNodeContainersInfo(ch chan *types.ReallocResourceMessage, nodeContainersInfo nodeContainers, cpu float64, memory int64, volumes types.VolumeBindings) (volCPUMemNodeContainers, map[string]types.VolumeBindings) {
	volCPUMemNodeContainersInfo := volCPUMemNodeContainers{}
	hardVbsForContainer := map[string]types.VolumeBindings{}
	for nodename, containers := range nodeContainersInfo {
		for _, container := range containers {
			if err := calVolCPUMemNodeContainerInfo(nodename, container, cpu, memory, volumes, volCPUMemNodeContainersInfo, hardVbsForContainer); err != nil {
				ch <- &types.ReallocResourceMessage{
					ContainerID: container.ID,
					Error:       err,
				}
			}
		}
	}
	return volCPUMemNodeContainersInfo, hardVbsForContainer
}

func (c *Calcium) doReallocContainer(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainersInfo nodeContainers,
	cpu float64, memory int64, volumes types.VolumeBindings,
	bindCPUOpt types.BindCPUOptions) {

	volCPUMemNodeContainersInfo, hardVbsForContainer := calVolCPUMemNodeContainersInfo(ch, nodeContainersInfo, cpu, memory, volumes)

	for newAutoVol, cpuMemNodeContainersInfo := range volCPUMemNodeContainersInfo {
		for newCPU, memNodesContainers := range cpuMemNodeContainersInfo {
			for newMemory, nodesContainers := range memNodesContainers {
				for nodename, containers := range nodesContainers {
					if err := c.withNodeLocked(ctx, nodename, func(node *types.Node) error {
						// 把记录的 CPU 还回去，变成新的可用资源
						// 把记录的 Memory 还回去，变成新的可用资源
						containerWithCPUBind := 0
						for _, container := range containers {
							// 不更新 etcd，内存计算
							node.CPU.Add(container.CPU)
							node.SetCPUUsed(container.Quota, types.DecrUsage)
							node.Volume.Add(container.VolumePlan.IntoVolumeMap())
							node.StorageCap += container.Storage
							node.SetVolumeUsed(container.VolumePlan.IntoVolumeMap().Total(), types.DecrUsage)
							node.MemCap += container.Memory
							if nodeID := node.GetNUMANode(container.CPU); nodeID != "" {
								node.IncrNUMANodeMemory(nodeID, container.Memory)
							}
							// cpu 绑定判断
							switch bindCPUOpt {
							case types.BindCPUOptionKeep:
								if len(container.CPU) > 0 {
									containerWithCPUBind++
								}
							case types.BindCPUOptionBind:
								containerWithCPUBind++
							case types.BindCPUOptionUnbind:
								containerWithCPUBind = 0
							}
						}

						// 检查内存
						if newMemory != 0 {
							if cap := int(node.MemCap / newMemory); cap < len(containers) {
								return types.NewDetailedErr(types.ErrInsufficientRes, node.Name)
							}
						}
						cpusets := make([]types.CPUMap, containerWithCPUBind)
						unlimitedCPUSet := types.CPUMap{}
						for i := 0; i < len(node.InitCPU); i++ {
							unlimitedCPUSet[strconv.Itoa(i)] = 0
						}
						for i := range cpusets {
							cpusets[i] = unlimitedCPUSet
						}
						// 按照 Node one by one 重新计算可以部署多少容器
						if containerWithCPUBind > 0 && newCPU != 0 {
							nodesInfo := []types.NodeInfo{{Name: node.Name, CPUMap: node.CPU, MemCap: node.MemCap}}
							// 重新计算需求
							_, nodeCPUPlans, total, err := c.scheduler.SelectCPUNodes(nodesInfo, newCPU, newMemory)
							if err != nil {
								return err
							}
							// 这里只有1个节点，肯定会出现1个节点的解决方案
							if total < containerWithCPUBind || len(nodeCPUPlans) != 1 {
								return types.ErrInsufficientRes
							}
							// 得到最终方案
							cpusets = nodeCPUPlans[node.Name][:containerWithCPUBind]
						}

						newResource := &enginetypes.VirtualizationResource{
							Quota:  newCPU,
							Memory: newMemory,
						}

						if err := c.updateContainersResources(ctx, ch, node, containers, newResource, cpusets, hardVbsForContainer, newAutoVol, bindCPUOpt); err != nil {
							return err
						}

						if err := c.store.UpdateNode(ctx, node); err != nil {
							log.Errorf("[doReallocContainer] Realloc finish but update node %s failed %s", node.Name, err)
							litter.Dump(node)
						}
						return nil
					}); err != nil {
						for _, container := range containers {
							log.Errorf("[doReallocContainer] Realloc container %v failed: %v", container.ID, err)
							ch <- &types.ReallocResourceMessage{
								ContainerID: container.ID,
								Error:       err,
							}
						}
					}
				}
			}
		}
	}
}

func (c *Calcium) updateContainersResources(ctx context.Context, ch chan *types.ReallocResourceMessage,
	node *types.Node, containers []*types.Container,
	newResource *enginetypes.VirtualizationResource,
	cpusets []types.CPUMap, hardVbsForContainer map[string]types.VolumeBindings, newAutoVol string, bindCPUOpt types.BindCPUOptions) error {

	autoVbs, _ := types.MakeVolumeBindings(strings.Split(newAutoVol, ","))
	planForContainers, err := c.reallocVolume(node, containers, autoVbs)
	if err != nil {
		return err
	}

	for _, container := range containers {
		// 情况1，原来就有绑定cpu的，保持不变
		if (len(container.CPU) > 0 && bindCPUOpt == types.BindCPUOptionKeep) ||
			// 情况2，有绑定指令，不管之前有没有cpuMap，都分配
			bindCPUOpt == types.BindCPUOptionBind {
			newResource.CPU = cpusets[0]
			newResource.NUMANode = node.GetNUMANode(cpusets[0])
			cpusets = cpusets[1:]
		}

		if newAutoVol != "" {
			newResource.VolumePlan = planForContainers[container].ToLiteral()
			newResource.Volumes = append(newResource.Volumes, autoVbs.ToStringSlice(false, false)...)
		}

		newVbs, _ := types.MakeVolumeBindings(newResource.Volumes)
		if !newVbs.IsEqual(container.Volumes) {
			newResource.VolumeChanged = true
		}

		newResource.SoftLimit = container.SoftLimit
		newResource.Volumes = hardVbsForContainer[container.ID].ToStringSlice(false, false)

		ch <- &types.ReallocResourceMessage{
			ContainerID: container.ID,
			Error:       c.updateResource(ctx, node, container, newResource),
		}
	}
	return nil
}

func (c *Calcium) updateResource(ctx context.Context, node *types.Node, container *types.Container, newResource *enginetypes.VirtualizationResource) error {
	if err := node.Engine.VirtualizationUpdateResource(ctx, container.ID, newResource); err == nil {
		oldVolumes := container.Volumes
		container.CPU = newResource.CPU
		container.Quota = newResource.Quota
		container.Memory = newResource.Memory
		container.Volumes, _ = types.MakeVolumeBindings(newResource.Volumes)
		container.VolumePlan = types.MustToVolumePlan(newResource.VolumePlan)
		container.Storage += container.Volumes.TotalSize() - oldVolumes.TotalSize()
	} else {
		log.Errorf("[updateResource] When Realloc container, VirtualizationUpdateResource %s failed %v", container.ID, err)
		return err
	}
	// 成功失败都需要修改 node 的占用
	// 成功的话，node 占用为新资源
	// 失败的话，node 占用为老资源
	node.CPU.Sub(container.CPU)
	node.SetCPUUsed(container.Quota, types.IncrUsage)
	node.Volume.Sub(container.VolumePlan.IntoVolumeMap())
	node.SetVolumeUsed(container.VolumePlan.IntoVolumeMap().Total(), types.IncrUsage)
	node.StorageCap -= container.Storage
	node.MemCap -= container.Memory
	if nodeID := node.GetNUMANode(container.CPU); nodeID != "" {
		node.DecrNUMANodeMemory(nodeID, container.Memory)
	}
	// 更新 container 元数据
	if err := c.store.UpdateContainer(ctx, container); err != nil {
		log.Errorf("[updateResource] Realloc finish but update container %s failed %v", container.ID, err)

		return err
	}
	return nil
}

func (c *Calcium) reallocVolume(node *types.Node, containers []*types.Container, vbs types.VolumeBindings) (plans map[*types.Container]types.VolumePlan, err error) {
	if len(vbs) == 0 {
		return
	}

	nodesInfo := []types.NodeInfo{{Name: node.Name, VolumeMap: node.Volume, InitVolumeMap: node.InitVolume}}
	_, nodeVolumePlans, total, err := c.scheduler.SelectVolumeNodes(nodesInfo, vbs)
	if err != nil {
		return
	}
	if total < len(containers) || len(nodeVolumePlans) != 1 {
		return nil, types.ErrInsufficientVolume
	}

	// select plans, existing bindings stick to the current devices
	plans = map[*types.Container]types.VolumePlan{}

Searching:
	for _, plan := range nodeVolumePlans[node.Name] {
		for _, container := range containers {
			if _, ok := plans[container]; !ok && plan.Compatible(container.VolumePlan) {
				plans[container] = plan
				if len(plans) == len(containers) {
					break Searching
				}
				break
			}
		}
	}
	if len(plans) < len(containers) {
		return nil, errors.Wrap(types.ErrInsufficientVolume, "reallocated volumes not compatible to existing ones")
	}

	return
}
