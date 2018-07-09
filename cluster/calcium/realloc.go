package calcium

import (
	"context"
	"fmt"
	"sync"

	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

//ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, IDs []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error) {
	// TODO 大量容器 Get 的时候有性能问题
	containers, err := c.store.GetContainers(ctx, IDs)
	if err != nil {
		return nil, err
	}

	// Pod-Node-Containers 三元组
	containersInfo := map[*types.Pod]NodeContainers{}
	// Pod-cpu-mem-node-containers 四元组
	cpuMemContainersInfo := map[*types.Pod]CPUMemNodeContainers{}
	// Raw resource container Node-Containers
	rawResourceContainers := NodeContainers{}
	nodeCache := map[string]*types.Node{}

	for _, container := range containers {
		if _, ok := nodeCache[container.Nodename]; !ok {
			node, err := c.GetNode(ctx, container.Podname, container.Nodename)
			if err != nil {
				return nil, err
			}
			nodeCache[container.Nodename] = node
		}
		node := nodeCache[container.Nodename]

		if container.RawResource {
			if _, ok := rawResourceContainers[node]; !ok {
				rawResourceContainers[node] = []*types.Container{}
			}
			rawResourceContainers[node] = append(rawResourceContainers[node], container)
			continue
		}

		pod, err := c.store.GetPod(ctx, container.Podname)
		if err != nil {
			return nil, err
		}

		if _, ok := containersInfo[pod]; !ok {
			containersInfo[pod] = NodeContainers{}
		}
		if _, ok := containersInfo[pod][node]; !ok {
			containersInfo[pod][node] = []*types.Container{}
		}
		containersInfo[pod][node] = append(containersInfo[pod][node], container)

		if pod.Favor == scheduler.CPU_PRIOR {
			if _, ok := cpuMemContainersInfo[pod]; !ok {
				cpuMemContainersInfo[pod] = CPUMemNodeContainers{}
			}
			podCPUMemContainersInfo := cpuMemContainersInfo[pod]
			newCPURequire := calculateCPUUsage(c.config.Scheduler.ShareBase, container) + cpu
			newMemRequire := container.Memory + mem
			if newCPURequire < 0.0 {
				return nil, fmt.Errorf("CPU can not below zero")
			}
			if newMemRequire < 0 {
				return nil, fmt.Errorf("Memory can not below zero")
			}

			// init data
			if _, ok := podCPUMemContainersInfo[newCPURequire]; !ok {
				podCPUMemContainersInfo[newCPURequire] = map[int64]NodeContainers{}
			}
			if _, ok := podCPUMemContainersInfo[newCPURequire][newMemRequire]; !ok {
				podCPUMemContainersInfo[newCPURequire][newMemRequire] = NodeContainers{}
			}
			if _, ok := podCPUMemContainersInfo[newCPURequire][newMemRequire][node]; !ok {
				podCPUMemContainersInfo[newCPURequire][newMemRequire][node] = []*types.Container{}
			}
			podCPUMemContainersInfo[newCPURequire][newMemRequire][node] = append(podCPUMemContainersInfo[newCPURequire][newMemRequire][node], container)
		}
	}

	ch := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo) + len(rawResourceContainers))

		// deal with container with raw resource
		for node, containers := range rawResourceContainers {
			go func(node *types.Node, containers []*types.Container) {
				defer wg.Done()
				c.doUpdateContainerWithMemoryPrior(ctx, ch, node.Podname, node, containers, cpu, mem)
			}(node, containers)
		}

		// deal with normal container
		for pod, nodeContainers := range containersInfo {
			if pod.Favor == scheduler.CPU_PRIOR {
				cpuMemNodeContainersInfo := cpuMemContainersInfo[pod]
				go func(pod *types.Pod, cpuMemNodeContainersInfo CPUMemNodeContainers) {
					defer wg.Done()
					c.reallocContainersWithCPUPrior(ctx, ch, pod, cpuMemNodeContainersInfo)
				}(pod, cpuMemNodeContainersInfo)
				continue
			}
			go func(pod *types.Pod, nodeContainers NodeContainers) {
				defer wg.Done()
				c.reallocContainerWithMemoryPrior(ctx, ch, pod, nodeContainers, cpu, mem)
			}(pod, nodeContainers)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (c *Calcium) checkNodesMemory(ctx context.Context, podname string, nodeContainers NodeContainers, memory int64) error {
	lock, err := c.Lock(ctx, podname, c.config.LockTimeout)
	if err != nil {
		return err
	}
	defer lock.Unlock(ctx)
	for node, containers := range nodeContainers {
		if cap := int(node.MemCap / memory); cap < len(containers) {
			return fmt.Errorf("Not enough resource %s", node.Name)
		}
		if err := c.store.UpdateNodeResource(ctx, podname, node.Name, types.CPUMap{}, int64(len(containers))*memory, "-"); err != nil {
			return err
		}
	}
	return nil
}

func (c *Calcium) reallocContainerWithMemoryPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainers NodeContainers,
	cpu float64, memory int64) {

	if memory > 0 {
		if err := c.checkNodesMemory(ctx, pod.Name, nodeContainers, memory); err != nil {
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

func (c *Calcium) doUpdateContainerWithMemoryPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	podname string,
	node *types.Node,
	containers []*types.Container,
	cpu float64, memory int64) {

	for _, container := range containers {
		containerJSON, err := container.Inspect(ctx)
		if err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] get container failed %v", err)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}

		cpuQuota := int64(cpu * float64(CpuPeriodBase))
		newCPUQuota := containerJSON.HostConfig.CPUQuota + cpuQuota
		newMemory := containerJSON.HostConfig.Memory + memory
		if newCPUQuota <= 0 || newMemory <= minMemory {
			log.Warnf("[doUpdateContainerWithMemoryPrior] new resource invaild %s, %d, %d", containerJSON.ID, newCPUQuota, newMemory)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}
		newCPU := float64(newCPUQuota) / float64(CpuPeriodBase)
		log.Debugf("[doUpdateContainerWithMemoryPrior] quota:%d, cpu: %f, mem: %d", newCPUQuota, newCPU, newMemory)

		// CPUQuota not cpu
		newResource := makeMemoryPriorSetting(newMemory, newCPU)
		updateConfig := enginecontainer.UpdateConfig{Resources: newResource}
		if err := updateContainer(ctx, containerJSON.ID, node, updateConfig); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update container failed %v, %s", err, containerJSON.ID)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			// 如果是增加内存，失败的时候应该把内存还回去
			if memory > 0 && !container.RawResource {
				if err := c.store.UpdateNodeResource(ctx, podname, node.Name, types.CPUMap{}, memory, "+"); err != nil {
					log.Errorf("[doUpdateContainerWithMemoryPrior] failed to set mem back %s", containerJSON.ID)
				}
			}
			continue
		}
		// 如果是要降低内存，当执行成功的时候需要把内存还回去
		if memory < 0 && !container.RawResource {
			if err := c.store.UpdateNodeResource(ctx, podname, node.Name, types.CPUMap{}, -memory, "+"); err != nil {
				log.Errorf("[doUpdateContainerWithMemoryPrior] failed to set mem back %s", containerJSON.ID)
			}
		}

		container.Memory = newMemory
		if err := c.store.AddContainer(ctx, container); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update container meta failed %v", err)
			// 立即中断
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			return
		}
		ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: true}
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
	for requireCPU, memNodesInfo := range cpuMemNodeContainersInfo {
		for requireMemory, nodesInfo := range memNodesInfo {
			for node, containers := range nodesInfo {
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
							CpuMap: node.CPU,
							MemCap: node.MemCap,
						},
						Name: node.Name,
					},
				}

				// 重新计算需求
				nodesInfo, nodePlans, total, err := c.scheduler.SelectCPUNodes(nodesInfo, requireCPU, requireMemory)
				if err != nil {
					// 满足不了，恢复之前的资源
					c.resetContainerResource(ctx, podname, node.Name, containers)
					return nil, err
				}
				nodesInfo, err = c.scheduler.EachDivision(nodesInfo, need, total)
				if err != nil {
					// 满足不了，恢复之前的资源
					c.resetContainerResource(ctx, podname, node.Name, containers)
					return nil, err
				}
				result, changed := c.scheduler.MakeCPUPlan(nodesInfo, nodePlans)

				// 这里 need 一定等于 len(containersCPUMaps)
				nodeCPUMap, isChanged := changed[node.Name]
				containersCPUMap, hasResult := result[node.Name]
				if isChanged && hasResult {
					node.CPU = nodeCPUMap
					node.MemCap = node.MemCap - requireMemory*int64(need)
					if err := c.store.UpdateNode(ctx, node); err != nil {
						return nil, err
					}
					if _, ok := cpuMemNodesMap[requireCPU]; !ok {
						cpuMemNodesMap[requireCPU] = map[int64]NodeCPUMap{}
					}
					if _, ok := cpuMemNodesMap[requireCPU][requireMemory]; !ok {
						cpuMemNodesMap[requireCPU][requireMemory] = NodeCPUMap{}
					}
					cpuMemNodesMap[requireCPU][requireMemory][node] = containersCPUMap
				}
			}
		}
	}
	return cpuMemNodesMap, nil
}

func (c *Calcium) reallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	cpuMemNodeContainersInfo CPUMemNodeContainers) {

	cpuMemNodesMap, err := c.reallocNodesCPUMem(ctx, pod.Name, cpuMemNodeContainersInfo)
	if err != nil {
		log.Errorf("[reallocContainersWithCPUPrior] realloc cpu resource failed %v", err)
		for _, memNodeMap := range cpuMemNodeContainersInfo {
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
	for cpu, memNodeResult := range cpuMemNodesMap {
		c.doReallocContainersWithCPUPrior(ctx, ch, pod.Name, memNodeResult, cpuMemNodeContainersInfo[cpu])
	}
}

func (c *Calcium) doReallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	podname string,
	memNodeResult map[int64]NodeCPUMap,
	memNodeContainers map[int64]NodeContainers,
) {

	for requireMemory, nodesCPUResult := range memNodeResult {
		nodeContainers := memNodeContainers[requireMemory]
		for node, cpuset := range nodesCPUResult {
			containers := nodeContainers[node]
			for index, container := range containers {
				quota := cpuset[index]
				resource := makeCPUPriorSetting(c.config.Scheduler.ShareBase, quota, requireMemory)
				updateConfig := enginecontainer.UpdateConfig{Resources: resource}
				if err := updateContainer(ctx, container.ID, node, updateConfig); err != nil {
					log.Errorf("[doReallocContainersWithCPUPrior] update container failed %v", err)
					// TODO 这里理论上是可以恢复 CPU 占用表的，一来我们知道新的占用是怎样，二来我们也晓得老的占用是啥样
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				}

				container.CPU = quota
				container.Memory = requireMemory
				if err := c.store.AddContainer(ctx, container); err != nil {
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
