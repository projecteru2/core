package calcium

import (
	"context"
	"sync"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/lock"

	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ReallocResource allow realloc container resource
func (c *Calcium) ReallocResource(ctx context.Context, IDs []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error) {
	ch := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch)
		// Container objs and their locks
		containers, _, containerLocks, err := c.LockAndGetContainers(ctx, IDs)
		if err != nil {
			log.Errorf("[ReallocResource] Lock and get containers failed %v", err)
			return
		}
		defer c.UnlockAll(ctx, containerLocks)
		// Pod-Node-Containers
		containersInfo := map[*types.Pod]NodeContainers{}
		// Pod cache
		podCache := map[string]*types.Pod{}
		// Node locks
		nodeLocks := map[string]lock.DistributedLock{}
		defer c.UnlockAll(ctx, nodeLocks)
		// Node cache
		nodeCache := map[string]*types.Node{}

		for _, container := range containers {
			pod, ok := podCache[container.Podname]
			if !ok {
				pod, err := c.store.GetPod(ctx, container.Podname)
				if err != nil {
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					continue
				}
				podCache[container.Podname] = pod
				containersInfo[pod] = NodeContainers{}
			}
			// 没锁过
			if _, ok := nodeLocks[container.Nodename]; !ok {
				node, nodeLock, err := c.LockAndGetNode(ctx, container.Podname, container.Nodename)
				if err != nil {
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
					continue
				}
				nodeLocks[container.Nodename] = nodeLock
				containersInfo[pod][node] = []*types.Container{container}
				nodeCache[container.Nodename] = node
				continue
			}
			// 锁过
			node := nodeCache[container.Nodename]
			containersInfo[pod][node] = append(containersInfo[pod][node], container)
		}

		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		// deal with normal container
		for pod, nodeContainers := range containersInfo {
			switch pod.Favor {
			case scheduler.MEMORY_PRIOR:
				go func(NodeContainers) {
					defer wg.Done()
					c.reallocContainerWithMemoryPrior(ctx, ch, nodeContainers, cpu, mem)
				}(nodeContainers)
			case scheduler.CPU_PRIOR:
				go func(nodeContainers NodeContainers) {
					defer wg.Done()
					c.reallocContainersWithCPUPrior(ctx, ch, nodeContainers, cpu, mem)
				}(nodeContainers)
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
	nodeContainers NodeContainers,
	cpu float64, memory int64) {

	// 不考虑 memory < 0 对于系统而言，这时候 realloc 只不过使得 node 记录的内存 > 容器拥有内存总和，并不会 OOM
	if memory > 0 {
		if err := c.reallocNodesMemory(ctx, nodeContainers, memory); err != nil {
			log.Errorf("[reallocContainerWithMemoryPrior] realloc memory failed %v", err)
			for _, containers := range nodeContainers {
				for _, container := range containers {
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				}
			}
			return
		}
	}

	c.doUpdateContainerWithMemoryPrior(ctx, ch, nodeContainers, cpu, memory)
}

// 只考虑增量 memory 的消耗
func (c *Calcium) reallocNodesMemory(ctx context.Context, nodeContainers NodeContainers, memory int64) error {
	// 只 check 增量情况下是否满足所需
	for node, containers := range nodeContainers {
		if cap := int(node.MemCap / memory); cap < len(containers) {
			return types.NewDetailedErr(types.ErrInsufficientRes, node.Name)
		}
	}
	return nil
}

func (c *Calcium) doUpdateContainerWithMemoryPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	nodeContainers NodeContainers,
	cpu float64, memory int64) {
	for node, containers := range nodeContainers {
		for _, container := range containers {
			newCPU := container.Quota + cpu
			newMemory := container.Memory + memory
			// 内存不能低于 4MB
			if newCPU <= 0 || newMemory <= minMemory {
				log.Errorf("[doUpdateContainerWithMemoryPrior] new resource invaild %s, %f, %d", container.ID, newCPU, newMemory)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			log.Debugf("[doUpdateContainerWithMemoryPrior] container %s: cpu: %f, mem: %d", container.ID, newCPU, newMemory)
			// CPUQuota not cpu
			newResource := makeResourceSetting(newCPU, newMemory, nil, container.SoftLimit)
			updateConfig := enginecontainer.UpdateConfig{Resources: newResource}
			if _, err := node.Engine.ContainerUpdate(ctx, container.ID, updateConfig); err != nil {
				log.Errorf("[doUpdateContainerWithMemoryPrior] update container failed %v, %s", err, container.ID)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			// 成功的时候应该记录内存变动
			if memory > 0 {
				node.MemCap -= memory
			} else {
				node.MemCap += memory
			}
			// 更新容器元信息
			container.Quota = newCPU
			container.Memory = newMemory
			if err := c.store.UpdateContainer(ctx, container); err != nil {
				log.Warnf("[doUpdateContainerWithMemoryPrior] update container %s failed %v", container.ID, err)
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				continue
			}
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
		}
		if err := c.store.UpdateNode(ctx, node); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update node %s failed %s", node.Name, err)
			litter.Dump(node)
			return
		}
	}
}

func (c *Calcium) reallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
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

	cpuMemNodesMap, err := c.reallocNodesCPUMem(ctx, cpuMemNodeContainers)
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

	c.doReallocContainersWithCPUPrior(ctx, ch, cpuMemNodesMap, cpuMemNodeContainers)
}

func (c *Calcium) reallocNodesCPUMem(
	ctx context.Context,
	cpuMemNodeContainersInfo CPUMemNodeContainers,
) (CPUMemNodeContainersMap, error) {
	// 不做实际的 node 分配，反正已经锁住了，只计算可能性
	cpuMemNodesMap := CPUMemNodeContainersMap{}
	for requireCPU, memNodesContainers := range cpuMemNodeContainersInfo {
		for requireMemory, nodesContainers := range memNodesContainers {
			for node, containers := range nodesContainers {
				// 把记录的 CPU 还回去，变成新的可用资源
				// 把记录的 Mem 还回去，变成新的可用资源
				for _, container := range containers {
					// 不更新 etcd，内存计算
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
					return nil, err
				}
				// 这里只有1个节点，肯定会出现1个节点的解决方案
				if total < need || len(nodeCPUPlans) != 1 {
					return nil, types.ErrInsufficientRes
				}

				if _, ok := cpuMemNodesMap[requireCPU]; !ok {
					cpuMemNodesMap[requireCPU] = map[int64]NodeCPUMap{}
				}
				if _, ok := cpuMemNodesMap[requireCPU][requireMemory]; !ok {
					cpuMemNodesMap[requireCPU][requireMemory] = NodeCPUMap{}
				}
				cpuMemNodesMap[requireCPU][requireMemory][node] = nodeCPUPlans[node.Name][:need]
			}
		}
	}
	return cpuMemNodesMap, nil
}

func (c *Calcium) doReallocContainersWithCPUPrior(
	ctx context.Context,
	ch chan *types.ReallocResourceMessage,
	cpuMemNodesMap CPUMemNodeContainersMap,
	cpuMemNodeContainers CPUMemNodeContainers,
) {
	for newCPU, memNodeResult := range cpuMemNodesMap {
		for newMem, nodesCPUResult := range memNodeResult {
			nodeContainers := cpuMemNodeContainers[newCPU][newMem]
			for node, cpuset := range nodesCPUResult {
				containers := nodeContainers[node]
				for index, container := range containers {
					cpuPlan := cpuset[index]
					resource := makeResourceSetting(newCPU, newMem, cpuPlan, container.SoftLimit)
					updateConfig := enginecontainer.UpdateConfig{Resources: resource}
					if _, err := node.Engine.ContainerUpdate(ctx, container.ID, updateConfig); err != nil {
						log.Errorf("[doReallocContainersWithCPUPrior] update container failed %v", err)
						ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
						continue
					}
					// 成功的时候应该记录变动
					node.CPU.Sub(cpuPlan)
					node.MemCap -= newMem
					container.CPU = cpuPlan
					container.Quota = newCPU
					container.Memory = newMem
					if err := c.store.UpdateContainer(ctx, container); err != nil {
						log.Warnf("[doReallocContainersWithCPUPrior] update container %s failed %v", container.ID, err)
						ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
						continue
					}
					ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
				}
				if err := c.store.UpdateNode(ctx, node); err != nil {
					log.Errorf("[doReallocContainersWithCPUPrior] update node %s failed %s", node.Name, err)
					litter.Dump(node)
					return
				}
			}
		}
	}
}
