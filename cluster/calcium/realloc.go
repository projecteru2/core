package calcium

import (
	"context"
	"sort"
	"strings"
	"sync"

	"github.com/sanity-io/litter"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// nodename -> container list
type nodeContainers map[string][]*types.Container

// cpu:memory
type volCpuMemNodeContainers map[string]map[float64]map[int64]nodeContainers

// ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, IDs []string, cpu float64, memory int64, volumes []string) (chan *types.ReallocResourceMessage, error) {
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
						ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
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
					c.doReallocContainer(ctx, ch, pod, nodeContainersInfo, cpu, memory, volumes)
				}(pod, nodeContainersInfo)
			}
			wg.Wait()
			return nil
		}); err != nil {
			log.Errorf("[ReallocResource] Realloc failed %v", err)
			for _, ID := range IDs {
				ch <- &types.ReallocResourceMessage{ContainerID: ID, Success: false}
			}
		}
	}()
	return ch, nil
}

func (c *Calcium) doReallocContainer(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainersInfo nodeContainers,
	cpu float64, memory int64, volumes []string) {

	volCpuMemNodeContainersInfo := volCpuMemNodeContainers{}
	for nodename, containers := range nodeContainersInfo {
		for _, container := range containers {
			newCPU := utils.Round(container.Quota + cpu)
			newMem := container.Memory + memory
			if newCPU <= 0 || newMem <= 0 {
				log.Errorf("[doReallocContainer] New resource invaild %s, %f, %d", container.ID, newCPU, newMem)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			vol, err := mergeAutoVolumeRequests(container.Volumes, volumes)
			if err != nil {
				log.Errorf("[doReallocContainer] New resource invalid %s, %v, %v", container.ID, vol, err)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			sort.Slice(vol, func(i, j int) bool { return vol[i] < vol[j] })
			newVol := strings.Join(vol, ",")

			if _, ok := volCpuMemNodeContainersInfo[newVol]; !ok {
				volCpuMemNodeContainersInfo[newVol] = map[float64]map[int64]nodeContainers{}
			}
			if _, ok := volCpuMemNodeContainersInfo[newVol][newCPU]; !ok {
				volCpuMemNodeContainersInfo[newVol][newCPU] = map[int64]nodeContainers{}
			}
			if _, ok := volCpuMemNodeContainersInfo[newVol][newCPU][newMem]; !ok {
				volCpuMemNodeContainersInfo[newVol][newCPU][newMem] = nodeContainers{}
			}
			if _, ok := volCpuMemNodeContainersInfo[newVol][newCPU][newMem][nodename]; !ok {
				volCpuMemNodeContainersInfo[newVol][newCPU][newMem][nodename] = []*types.Container{}
			}
			volCpuMemNodeContainersInfo[newVol][newCPU][newMem][nodename] = append(volCpuMemNodeContainersInfo[newVol][newCPU][newMem][nodename], container)
		}
	}

	for newVol, cpuMemNodeContainersInfo := range volCpuMemNodeContainersInfo {
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
							node.Volume.Add(container.VolumePlan.Merge())
							node.SetVolumeUsed(container.VolumePlan.Merge().Total(), types.DecrUsage)
							node.MemCap += container.Memory
							if nodeID := node.GetNUMANode(container.CPU); nodeID != "" {
								node.IncrNUMANodeMemory(nodeID, container.Memory)
							}
							if len(container.CPU) > 0 {
								containerWithCPUBind++
							}
						}
						// 检查内存
						if cap := int(node.MemCap / newMemory); cap < len(containers) {
							return types.NewDetailedErr(types.ErrInsufficientRes, node.Name)
						}
						var cpusets []types.CPUMap
						// 按照 Node one by one 重新计算可以部署多少容器
						if containerWithCPUBind > 0 {
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

						var volumePlans []types.VolumePlan
						// newVol won't be empty as long as existing container bound volumes before or realloc request requires new binding
						if newVol != "" {
							volumes := strings.Split(newVol, ",")
							nodesInfo := []types.NodeInfo{{Name: node.Name, VolumeMap: node.Volume}}
							_, nodeVolumePlans, total, err := c.scheduler.SelectVolumeNodes(nodesInfo, volumes)
							if err != nil {
								return err
							}
							if total < len(containers) || len(nodeVolumePlans) != 1 {
								return types.ErrInsufficientVolume
							}

							// select plans, existing bindings stick to the current devices
							planForContainer := map[string]types.VolumePlan{}
							for _, plan := range nodeVolumePlans[node.Name] {
								for _, container := range containers {
									if _, ok := planForContainer[container.ID]; !ok && plan.Compatible(container.VolumePlan) {
										volumePlans = append(volumePlans, plan)
										planForContainer[container.ID] = plan
										break
									}
								}
							}
							if len(volumePlans) < len(containers) {
								return types.ErrInsufficientVolume
							}
						}

						for idx, container := range containers {
							newResource := &enginetypes.VirtualizationResource{Quota: newCPU, Memory: newMemory, SoftLimit: container.SoftLimit, Volumes: volumes}
							if len(container.CPU) > 0 {
								newResource.CPU = cpusets[0]
								newResource.NUMANode = node.GetNUMANode(cpusets[0])
								cpusets = cpusets[1:]
							}

							if newVol != "" {
								newResource.VolumePlan = volumePlans[idx].ToLiteral()
							}

							updateSuccess := false
							setSuccess := false
							if err := node.Engine.VirtualizationUpdateResource(ctx, container.ID, newResource); err == nil {
								container.CPU = newResource.CPU
								container.Quota = newResource.Quota
								container.Memory = newResource.Memory
								container.Volumes = newResource.Volumes
								container.VolumePlan = types.ToVolumePlan(newResource.VolumePlan)
								updateSuccess = true
							} else {
								log.Errorf("[doReallocContainer] Realloc container %s failed %v", container.ID, err)
							}
							// 成功失败都需要修改 node 的占用
							// 成功的话，node 占用为新资源
							// 失败的话，node 占用为老资源
							node.CPU.Sub(container.CPU)
							node.SetCPUUsed(container.Quota, types.IncrUsage)
							node.Volume.Sub(container.VolumePlan.Merge())
							node.SetVolumeUsed(container.VolumePlan.Merge().Total(), types.IncrUsage)
							node.MemCap -= container.Memory
							if nodeID := node.GetNUMANode(container.CPU); nodeID != "" {
								node.DecrNUMANodeMemory(nodeID, container.Memory)
							}
							// 更新 container 元数据
							if err := c.store.UpdateContainer(ctx, container); err == nil {
								setSuccess = true
							} else {
								log.Errorf("[doReallocContainer] Realloc finish but update container %s failed %v", container.ID, err)
							}
							ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: updateSuccess && setSuccess}
						}
						if err := c.store.UpdateNode(ctx, node); err != nil {
							log.Errorf("[doReallocContainer] Realloc finish but update node %s failed %s", node.Name, err)
							litter.Dump(node)
						}
						return nil
					}); err != nil {
						for _, container := range containers {
							ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
						}
					}
				}
			}
		}
	}
}
