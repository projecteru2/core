package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/lock"

	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, IDs []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error) {
	containers, err := c.store.GetContainers(ctx, IDs)
	if err != nil {
		return nil, err
	}

	ch := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch)
		// Pod-Node-Containers 三元组
		containersInfo := map[*types.Pod]NodeContainers{}
		// container locked
		containerLocks := map[string]lock.DistributedLock{}
		defer func() {
			for ID, lock := range containerLocks {
				log.Debugf("[ReallocResource] Container %s unlocked", ID)
				lock.Unlock(ctx)
			}
		}()
		for _, container := range containers {
			container, _, lock, err := c.doLockContainer(ctx, container)
			if err != nil {
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			node := container.Node
			pod, err := c.store.GetPod(ctx, container.Podname)
			if err != nil {
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				lock.Unlock(ctx)
				continue
			}
			containerLocks[container.ID] = lock

			if _, ok := containersInfo[pod]; !ok {
				containersInfo[pod] = NodeContainers{}
			}
			if _, ok := containersInfo[pod][node]; !ok {
				containersInfo[pod][node] = []*types.Container{}
			}
			containersInfo[pod][node] = append(containersInfo[pod][node], container)
		}
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		// deal with normal container
		for pod, nodeContainers := range containersInfo {
			switch pod.Favor {
			case scheduler.CPU_PRIOR:
				go func(pod *types.Pod, nodeContainers NodeContainers) {
					defer wg.Done()
					c.reallocContainersWithCPUPrior(ctx, ch, pod, nodeContainers, cpu, mem)
				}(pod, nodeContainers)
			case scheduler.MEMORY_PRIOR:
				go func(pod *types.Pod, nodeContainers NodeContainers) {
					defer wg.Done()
					c.reallocContainerWithMemoryPrior(ctx, ch, pod, nodeContainers, cpu, mem)
				}(pod, nodeContainers)
			default:
				log.Errorf("[ReallocResource] %v not support yet", pod.Favor)
				go func(nodeContainers NodeContainers) {
					defer wg.Done()
					for _, containers := range nodeContainers {
						for _, container := range containers {
							ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
						}
					}
				}(nodeContainers)
			}
		}
		wg.Wait()
	}()
	return ch, nil
}

func (c *Calcium) reallocContainerWithMemoryPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainers NodeContainers,
	cpu float64, memory int64) {

	// 不考虑 memory < 0 对于系统而言，这时候 realloc 只不过使得 node 记录的内存 > 容器拥有内存总和，并不会 OOM
	if memory > 0 {
		if err := c.reallocNodesMemory(ctx, pod.Name, nodeContainers, memory); err != nil {
			log.Errorf("[reallocContainerWithMemoryPrior] realloc memory failed %v", err)
			for _, containers := range nodeContainers {
				for _, container := range containers {
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				}
			}
			return
		}
	}

	// 不并发操作了
	for node, containers := range nodeContainers {
		c.doUpdateContainerWithMemoryPrior(ctx, ch, pod.Name, node, containers, cpu, memory)
	}
}

// 只考虑增量 memory 的消耗
func (c *Calcium) reallocNodesMemory(ctx context.Context, podname string, nodeContainers NodeContainers, memory int64) error {
	lock, err := c.Lock(ctx, podname, c.config.LockTimeout)
	if err != nil {
		return err
	}
	defer lock.Unlock(ctx)
	for node, containers := range nodeContainers {
		if cap := int(node.MemCap / memory); cap < len(containers) {
			return types.NewDetailedErr(types.ErrInsufficientRes, node.Name)
		}
		if err := c.store.UpdateNodeResource(ctx, podname, node.Name, types.CPUMap{}, int64(len(containers))*memory, "-"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Calcium) doUpdateContainerWithMemoryPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	podname string,
	node *types.Node,
	containers []*types.Container,
	cpu float64, memory int64) {

	for _, container := range containers {
		newCPU := container.Quota + cpu
		newMemory := container.Memory + memory
		// 内存不能低于 4MB
		if newCPU <= 0 || newMemory <= minMemory {
			log.Errorf("[doUpdateContainerWithMemoryPrior] new resource invaild %s, %f, %d", container.ID, newCPU, newMemory)
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			continue
		}
		log.Debugf("[doUpdateContainerWithMemoryPrior] cpu: %f, mem: %d", newCPU, newMemory)
		// CPUQuota not cpu
		newResource := makeResourceSetting(newCPU, newMemory, nil, container.SoftLimit)
		updateConfig := enginecontainer.UpdateConfig{Resources: newResource}
		if _, err := node.Engine.ContainerUpdate(ctx, container.ID, updateConfig); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update container failed %v, %s", err, container.ID)
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			// 如果是增加内存，失败的时候应该把内存还回去
			if memory > 0 {
				if err := c.store.UpdateNodeResource(ctx, podname, node.Name, types.CPUMap{}, memory, "+"); err != nil {
					log.Errorf("[doUpdateContainerWithMemoryPrior] failed to set mem back %s", container.ID)
				}
			}
			continue
		}
		// 如果是要降低内存，当执行成功的时候需要把内存还回去
		if memory < 0 {
			if err := c.store.UpdateNodeResource(ctx, podname, node.Name, types.CPUMap{}, -memory, "+"); err != nil {
				log.Errorf("[doUpdateContainerWithMemoryPrior] failed to set mem back %s", container.ID)
			}
		}

		container.Quota = newCPU
		container.Memory = newMemory
		if err := c.store.UpdateContainer(ctx, container); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update container meta failed %v", err)
			// 立即中断
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			return
		}
		ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
	}
}

func (c *Calcium) reallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainers NodeContainers,
	cpu float64, memory int64) {

	cpuMemNodeContainers := CPUMemNodeContainers{}
	for node, containers := range nodeContainers {
		for _, container := range containers {
			newCPU := container.Quota + cpu
			newMem := container.Memory + memory
			if newCPU < 0 || newMem < minMemory {
				log.Errorf("[reallocContainersWithCPUPrior] new resource invaild %s, %f, %d", container.ID, newCPU, newMem)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			if _, ok := cpuMemNodeContainers[newCPU]; !ok {
				cpuMemNodeContainers[newCPU] = map[int64]NodeContainers{}
			}
			if _, ok := cpuMemNodeContainers[newCPU][newMem]; !ok {
				cpuMemNodeContainers[newCPU][newMem] = NodeContainers{}
			}
			if _, ok := cpuMemNodeContainers[newCPU][newMem][node]; !ok {
				cpuMemNodeContainers[newCPU][newMem][node] = []*types.Container{}
			}
			cpuMemNodeContainers[newCPU][newMem][node] = append(cpuMemNodeContainers[newCPU][newMem][node], container)
		}
	}

	cpuMemNodesMap, err := c.reallocNodesCPUMem(ctx, pod.Name, cpuMemNodeContainers)
	if err != nil {
		log.Errorf("[reallocContainersWithCPUPrior] realloc cpu resource failed %v", err)
		for _, memNodeMap := range cpuMemNodeContainers {
			for _, nodeInfoMap := range memNodeMap {
				for _, containers := range nodeInfoMap {
					for _, container := range containers {
						ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					}
				}
			}
		}
		return
	}

	// 不并发操作了
	for newCPU, memNodeResult := range cpuMemNodesMap {
		c.doReallocContainersWithCPUPrior(ctx, ch, newCPU, pod.Name, memNodeResult, cpuMemNodeContainers[newCPU])
	}
}

func (c *Calcium) reallocNodesCPUMem(
	ctx context.Context,
	podname string,
	cpuMemNodeContainersInfo CPUMemNodeContainers,
) (CPUMemNodeContainersMap, error) {

	lock, err := c.Lock(ctx, podname, c.config.LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock(ctx)

	// TODO too slow
	cpuMemNodesMap := CPUMemNodeContainersMap{}
	for requireCPU, memNodesContainers := range cpuMemNodeContainersInfo {
		for requireMemory, nodesContainers := range memNodesContainers {
			for node, containers := range nodesContainers {
				// 把记录的 CPU 还回去，变成新的可用资源
				// 把记录的 Mem 还回去，变成新的可用资源
				// 即便有并发操作，不影响 Create container 操作
				// 最坏情况就是 CPU/MEM 重叠了，可以外部纠正
				for _, container := range containers {
					// 更新 ETCD 数据
					if err := c.store.UpdateNodeResource(ctx, podname, node.Name, container.CPU, container.Memory, "+"); err != nil {
						return nil, err
					}
					// 更新 node 值
					node.CPU.Add(container.CPU)
					node.MemCap += container.Memory
				}

				// 按照 Node one by one 重新计算可以部署多少容器
				need := len(containers)
				nodesInfo := []types.NodeInfo{
					types.NodeInfo{
						CPUAndMem: types.CPUAndMem{
							CPUMap: node.CPU,
							MemCap: node.MemCap,
						},
						Name: node.Name,
					},
				}

				// 重新计算需求
				nodesInfo, nodeCPUPlans, total, err := c.scheduler.SelectCPUNodes(nodesInfo, requireCPU, requireMemory)
				if err != nil {
					c.resetContainerResource(ctx, podname, node.Name, containers)
					return nil, err
				}
				nodesInfo, err = c.scheduler.EachDivision(nodesInfo, need, 0)
				if err != nil {
					c.resetContainerResource(ctx, podname, node.Name, containers)
					return nil, err
				}
				// 这里只有1个节点，肯定会出现1个节点的解决方案
				if total < need || len(nodeCPUPlans) != 1 {
					return nil, types.ErrInsufficientRes
				}

				cpuCost := types.CPUMap{}
				memoryCost := requireMemory * int64(need)
				cpuList := nodeCPUPlans[node.Name][:need]
				for _, cpu := range cpuList {
					cpuCost.Add(cpu)
				}

				// 扣掉资源
				if err := c.store.UpdateNodeResource(ctx, podname, node.Name, cpuCost, memoryCost, "-"); err != nil {
					return nil, err
				}
				if _, ok := cpuMemNodesMap[requireCPU]; !ok {
					cpuMemNodesMap[requireCPU] = map[int64]NodeCPUMap{}
				}
				if _, ok := cpuMemNodesMap[requireCPU][requireMemory]; !ok {
					cpuMemNodesMap[requireCPU][requireMemory] = NodeCPUMap{}
				}
				cpuMemNodesMap[requireCPU][requireMemory][node] = cpuList
			}
		}
	}
	return cpuMemNodesMap, nil
}

func (c *Calcium) doReallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	quota float64,
	podname string,
	memNodeResult map[int64]NodeCPUMap,
	memNodeContainers map[int64]NodeContainers,
) {

	for requireMemory, nodesCPUResult := range memNodeResult {
		nodeContainers := memNodeContainers[requireMemory]
		for node, cpuset := range nodesCPUResult {
			containers := nodeContainers[node]
			for index, container := range containers {
				cpuPlan := cpuset[index]
				resource := makeResourceSetting(quota, requireMemory, cpuPlan, container.SoftLimit)
				updateConfig := enginecontainer.UpdateConfig{Resources: resource}
				if _, err := node.Engine.ContainerUpdate(ctx, container.ID, updateConfig); err != nil {
					log.Errorf("[doReallocContainersWithCPUPrior] update container failed %v", err)
					// TODO 这里理论上是可以恢复 CPU 占用表的，一来我们知道新的占用是怎样，二来我们也晓得老的占用是啥样
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				}

				container.CPU = cpuPlan
				container.Quota = quota
				container.Memory = requireMemory
				if err := c.store.UpdateContainer(ctx, container); err != nil {
					log.Errorf("[doReallocContainersWithCPUPrior] update container meta failed %v", err)
					// 立即中断
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					return
				}
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
			}
		}
	}
}

func (c *Calcium) resetContainerResource(ctx context.Context, podname, nodename string, containers []*types.Container) error {
	for _, container := range containers {
		if err := c.store.UpdateNodeResource(ctx, podname, nodename, container.CPU, container.Memory, "-"); err != nil {
			return err
		}
	}
	return nil
}
