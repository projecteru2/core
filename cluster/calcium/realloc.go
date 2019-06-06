package calcium

import (
	"context"
	"sync"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/utils"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// nodename -> container list
type nodeContainers map[string][]*types.Container

// nodename -> cpu list
type nodeCPUMap map[string][]types.CPUMap

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
				switch pod.Favor {
				case scheduler.MemoryPrior:
					go func(nodeContainersInfo nodeContainers) {
						defer wg.Done()
						c.doReallocContainerWithMemoryPrior(ctx, ch, pod, nodeContainersInfo, cpu, mem)
					}(nodeContainersInfo)
				case scheduler.CPUPrior:
					go func(nodeContainersInfo nodeContainers) {
						defer wg.Done()
						c.doReallocContainersWithCPUPrior(ctx, ch, pod, nodeContainersInfo, cpu, mem)
					}(nodeContainersInfo)
				default:
					log.Errorf("[ReallocResource] %v not support yet", pod.Favor)
					go func(nodeContainersInfo nodeContainers) {
						defer wg.Done()
						for _, containers := range nodeContainersInfo {
							for _, container := range containers {
								ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
							}
						}
					}(nodeContainersInfo)
				}
			}
			wg.Wait()
			return nil
		}); err != nil {
			log.Errorf("[ReallocResource] Realloc failed %v", err)
		}
	}()
	return ch, nil
}

func (c *Calcium) doReallocContainerWithMemoryPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainersInfo nodeContainers,
	cpu float64, memory int64) {
	for nodename, containers := range nodeContainersInfo {
		if err := c.withNodeLocked(ctx, pod.Name, nodename, func(node *types.Node) error {
			// 不考虑 memory < 0 对于系统而言，这时候 realloc 只不过使得 node 记录的内存 > 容器拥有内存总和，并不会 OOM
			if memory > 0 {
				if cap := int(node.MemCap / memory); cap < len(containers) {
					return types.NewDetailedErr(types.ErrInsufficientRes, node.Name)
				}
			}
			for _, container := range containers {
				newCPU := utils.Round(container.Quota + cpu)
				newMemory := container.Memory + memory
				// 内存不能低于 4MB
				if newCPU <= 0 || newMemory <= minMemory {
					log.Errorf("[doReallocContainerWithMemoryPrior] New resource invaild %s, %f, %d", container.ID, newCPU, newMemory)
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					continue
				}
				log.Debugf("[doReallocContainerWithMemoryPrior] Container %s: cpu: %f, mem: %d", container.ID, newCPU, newMemory)
				// CPUQuota not cpu
				newResource := &enginetypes.VirtualizationResource{
					CPU: nil, Quota: newCPU, Memory: newMemory, SoftLimit: container.SoftLimit}
				if err := node.Engine.VirtualizationUpdateResource(ctx, container.ID, newResource); err != nil {
					log.Errorf("[doReallocContainerWithMemoryPrior] Update container failed %v, %s", err, container.ID)
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					continue
				}
				// 记录内存变动
				if memory > 0 {
					node.MemCap -= memory
				} else {
					node.MemCap += -memory
				}
				// 记录CPU变动
				if cpu > 0 {
					node.SetCPUUsed(cpu, types.IncrUsage)
				} else {
					node.SetCPUUsed(cpu, types.DecrUsage)
				}
				// 更新容器元信息
				container.Quota = newCPU
				container.Memory = newMemory
				if err := c.store.UpdateContainer(ctx, container); err != nil {
					log.Warnf("[doUpdateContainerWithMemoryPrior] Realloc finish but update container %s failed %v", container.ID, err)
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					continue
				}
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
			}
			// update resource to storage
			if err := c.store.UpdateNode(ctx, node); err != nil {
				log.Errorf("[doUpdateContainerWithMemoryPrior] Realloc finish but update node %s failed %s", node.Name, err)
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

func (c *Calcium) doReallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainersInfo nodeContainers,
	cpu float64, memory int64) {

	cpuMemNodeContainersInfo := cpuMemNodeContainers{}
	for node, containers := range nodeContainersInfo {
		for _, container := range containers {
			newCPU := utils.Round(container.Quota + cpu)
			newMem := container.Memory + memory
			if newCPU < 0 || newMem < minMemory {
				log.Errorf("[reallocContainersWithCPUPrior] New resource invaild %s, %f, %d", container.ID, newCPU, newMem)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			if _, ok := cpuMemNodeContainersInfo[newCPU]; !ok {
				cpuMemNodeContainersInfo[newCPU] = map[int64]nodeContainers{}
			}
			if _, ok := cpuMemNodeContainersInfo[newCPU][newMem]; !ok {
				cpuMemNodeContainersInfo[newCPU][newMem] = nodeContainers{}
			}
			if _, ok := cpuMemNodeContainersInfo[newCPU][newMem][node]; !ok {
				cpuMemNodeContainersInfo[newCPU][newMem][node] = []*types.Container{}
			}
			cpuMemNodeContainersInfo[newCPU][newMem][node] = append(cpuMemNodeContainersInfo[newCPU][newMem][node], container)
		}
	}

	for requireCPU, memNodesContainers := range cpuMemNodeContainersInfo {
		for requireMemory, nodesContainers := range memNodesContainers {
			for nodename, containers := range nodesContainers {
				if err := c.withNodeLocked(ctx, pod.Name, nodename, func(node *types.Node) error {
					// 把记录的 CPU 还回去，变成新的可用资源
					// 把记录的 Mem 还回去，变成新的可用资源
					for _, container := range containers {
						// 不更新 etcd，内存计算
						node.CPU.Add(container.CPU)
						node.SetCPUUsed(container.Quota, types.DecrUsage)
						node.MemCap += container.Memory
					}
					// 按照 Node one by one 重新计算可以部署多少容器
					need := len(containers)
					nodesInfo := []types.NodeInfo{
						{
							Name:   node.Name,
							CPUMap: node.CPU,
							MemCap: node.MemCap,
						},
					}
					// 重新计算需求
					_, nodeCPUPlans, total, err := c.scheduler.SelectCPUNodes(nodesInfo, requireCPU, requireMemory)
					if err != nil {
						return err
					}
					// 这里只有1个节点，肯定会出现1个节点的解决方案
					if total < need || len(nodeCPUPlans) != 1 {
						return types.ErrInsufficientRes
					}
					// 得到最终方案
					cpuset := nodeCPUPlans[node.Name][:need]
					for index, container := range containers {
						cpuPlan := cpuset[index]
						newResource := &enginetypes.VirtualizationResource{
							CPU: cpuPlan.Map(), Quota: requireCPU, Memory: requireMemory, SoftLimit: container.SoftLimit}
						if err := node.Engine.VirtualizationUpdateResource(ctx, container.ID, newResource); err != nil {
							log.Errorf("[doReallocContainersWithCPUPrior] Update container failed %v", err)
							ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
							continue
						}
						// 成功的时候应该记录变动
						node.CPU.Sub(cpuPlan)
						node.SetCPUUsed(requireCPU, types.IncrUsage)
						node.MemCap -= requireMemory
						container.CPU = cpuPlan
						container.Quota = requireCPU
						container.Memory = requireMemory
						if err := c.store.UpdateContainer(ctx, container); err != nil {
							log.Warnf("[doReallocContainersWithCPUPrior] Realloc finish but update container %s failed %v", container.ID, err)
							ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
							continue
						}
						ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
					}
					if err := c.store.UpdateNode(ctx, node); err != nil {
						log.Errorf("[doReallocContainersWithCPUPrior] Realloc finish but update node %s failed %s", node.Name, err)
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
