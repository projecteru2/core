package calcium

import (
	"context"
	"sync"

	"github.com/sanity-io/litter"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// nodename -> container list
type nodeContainers map[string][]*types.Container

// cpu:mem
type cpuMemNodeContainers map[float64]map[int64]nodeContainers

// ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, IDs []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error) {
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
					c.doReallocContainer(ctx, ch, pod, nodeContainersInfo, cpu, mem)
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
	cpu float64, memory int64) {

	cpuMemNodeContainersInfo := cpuMemNodeContainers{}
	for nodename, containers := range nodeContainersInfo {
		for _, container := range containers {
			newCPU := utils.Round(container.Quota + cpu)
			newMem := container.Memory + memory
			if newCPU < 0 || newMem < minMemory {
				log.Errorf("[doReallocContainer] New resource invaild %s, %f, %d", container.ID, newCPU, newMem)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			if _, ok := cpuMemNodeContainersInfo[newCPU]; !ok {
				cpuMemNodeContainersInfo[newCPU] = map[int64]nodeContainers{}
			}
			if _, ok := cpuMemNodeContainersInfo[newCPU][newMem]; !ok {
				cpuMemNodeContainersInfo[newCPU][newMem] = nodeContainers{}
			}
			if _, ok := cpuMemNodeContainersInfo[newCPU][newMem][nodename]; !ok {
				cpuMemNodeContainersInfo[newCPU][newMem][nodename] = []*types.Container{}
			}
			cpuMemNodeContainersInfo[newCPU][newMem][nodename] = append(cpuMemNodeContainersInfo[newCPU][newMem][nodename], container)
		}
	}

	for newCPU, memNodesContainers := range cpuMemNodeContainersInfo {
		for newMemory, nodesContainers := range memNodesContainers {
			for nodename, containers := range nodesContainers {
				if err := c.withNodeLocked(ctx, pod.Name, nodename, func(node *types.Node) error {
					// 把记录的 CPU 还回去，变成新的可用资源
					// 把记录的 Mem 还回去，变成新的可用资源
					containerWithCPUBind := 0
					for _, container := range containers {
						// 不更新 etcd，内存计算
						node.CPU.Add(container.CPU)
						node.SetCPUUsed(container.Quota, types.DecrUsage)
						node.MemCap += container.Memory
						if nodeID := utils.GetNUMAMemoryNode(node, container.CPU); nodeID != "" {
							if _, ok := node.NUMAMem[nodeID]; ok {
								node.NUMAMem[nodeID] += container.Memory
							}
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

					for _, container := range containers {
						newResource := &enginetypes.VirtualizationResource{Quota: newCPU, Memory: newMemory, SoftLimit: container.SoftLimit}
						if len(container.CPU) > 0 {
							newResource.CPU = cpusets[0]
							newResource.NUMANode = utils.GetNUMAMemoryNode(node, newResource.CPU)
							cpusets = cpusets[1:]
						}
						updateSuccess := false
						setSuccess := false
						if err := node.Engine.VirtualizationUpdateResource(ctx, container.ID, newResource); err == nil {
							container.CPU = newResource.CPU
							container.Quota = newResource.Quota
							container.Memory = newResource.Memory
							updateSuccess = true
						} else {
							log.Errorf("[doReallocContainer] Realloc container %s failed %v", container.ID, err)
						}
						// 成功失败都需要修改 node 的占用
						// 成功的话，node 占用为新资源
						// 失败的话，node 占用为老资源
						node.CPU.Sub(container.CPU)
						node.SetCPUUsed(container.Quota, types.IncrUsage)
						node.MemCap -= container.Memory
						if nodeID := utils.GetNUMAMemoryNode(node, container.CPU); nodeID != "" {
							if _, ok := node.NUMAMem[nodeID]; ok {
								node.NUMAMem[nodeID] -= container.Memory
							}
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
