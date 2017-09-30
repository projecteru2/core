package calcium

import (
	"context"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginecontainer "github.com/docker/docker/api/types/container"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type NodeContainers map[*types.Node][]*types.Container
type NodeCPUMap map[*types.Node][]types.CPUMap
type CPUNodeContainers map[float64]NodeContainers
type CPUNodeContainersMap map[float64]NodeCPUMap

func (c *calcium) ReallocResource(ids []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error) {
	// TODO 大量容器 Get 的时候有性能问题
	containers, err := c.store.GetContainers(ids)
	if err != nil {
		return nil, err
	}

	// Pod-Node-Containers 三元组
	containersInfo := map[*types.Pod]NodeContainers{}
	// Pod-cpu-node-containers 四元组
	cpuContainersInfo := map[*types.Pod]CPUNodeContainers{}
	nodeCache := map[string]*types.Node{}

	for _, container := range containers {
		if _, ok := nodeCache[container.Nodename]; !ok {
			node, err := c.GetNode(container.Podname, container.Nodename)
			if err != nil {
				return nil, err
			}
			nodeCache[container.Nodename] = node
		}
		node := nodeCache[container.Nodename]

		pod, err := c.store.GetPod(container.Podname)
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

		if pod.Favor == types.CPU_PRIOR {
			if _, ok := cpuContainersInfo[pod]; !ok {
				cpuContainersInfo[pod] = CPUNodeContainers{}
			}
			podCPUContainersInfo := cpuContainersInfo[pod]
			newCPURequire := c.calculateCPUUsage(container) + cpu
			if newCPURequire < 0.0 {
				return nil, fmt.Errorf("Cpu can not below zero")
			}
			if _, ok := podCPUContainersInfo[newCPURequire]; !ok {
				podCPUContainersInfo[newCPURequire] = NodeContainers{}
				podCPUContainersInfo[newCPURequire][node] = []*types.Container{}
			}
			podCPUContainersInfo[newCPURequire][node] = append(podCPUContainersInfo[newCPURequire][node], container)
		}
	}

	ch := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod, nodeContainers := range containersInfo {
			if pod.Favor == types.CPU_PRIOR {
				nodeCPUContainersInfo := cpuContainersInfo[pod]
				go func(pod *types.Pod, nodeCPUContainersInfo CPUNodeContainers) {
					defer wg.Done()
					c.reallocContainersWithCPUPrior(ch, pod, nodeCPUContainersInfo, cpu, mem)
				}(pod, nodeCPUContainersInfo)
				continue
			}
			go func(pod *types.Pod, nodeContainers NodeContainers) {
				defer wg.Done()
				c.reallocContainerWithMemoryPrior(ch, pod, nodeContainers, cpu, mem)
			}(pod, nodeContainers)
		}
		wg.Wait()
	}()
	return ch, nil
}

func (c *calcium) checkNodesMemory(podname string, nodeContainers NodeContainers, memory int64) error {
	lock, err := c.Lock(podname, c.config.LockTimeout)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	for node, containers := range nodeContainers {
		if cap := int(node.MemCap / memory); cap < len(containers) {
			return fmt.Errorf("Not enough resource %s", node.Name)
		}
		if err := c.store.UpdateNodeMem(podname, node.Name, int64(len(containers))*memory, "-"); err != nil {
			return err
		}
	}
	return nil
}

func (c *calcium) reallocContainerWithMemoryPrior(
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodeContainers NodeContainers,
	cpu float64, memory int64) {

	if memory > 0 {
		if err := c.checkNodesMemory(pod.Name, nodeContainers, memory); err != nil {
			log.Errorf("[reallocContainerWithMemoryPrior] realloc memory failed %v", err)
			return
		}
	}

	// 不并发操作了
	for node, containers := range nodeContainers {
		c.doUpdateContainerWithMemoryPrior(ch, pod.Name, node, containers, cpu, memory)
	}
}

func (c *calcium) doUpdateContainerWithMemoryPrior(
	ch chan *types.ReallocResourceMessage,
	podname string,
	node *types.Node,
	containers []*types.Container,
	cpu float64, memory int64) {

	for _, container := range containers {
		containerJSON, err := container.Inspect()
		if err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] get container failed %v", err)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}

		cpuQuota := int64(cpu * float64(utils.CpuPeriodBase))
		newCPUQuota := containerJSON.HostConfig.CPUQuota + cpuQuota
		newMemory := containerJSON.HostConfig.Memory + memory
		if newCPUQuota <= 0 || newMemory <= minMemory {
			log.Warnf("[doUpdateContainerWithMemoryPrior] new resource invaild %s, %d, %d", containerJSON.ID, newCPUQuota, newMemory)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}
		newCPU := float64(newCPUQuota) / float64(utils.CpuPeriodBase)
		log.Debugf("[doUpdateContainerWithMemoryPrior] quota:%d, cpu: %f, mem: %d", newCPUQuota, newCPU, newMemory)

		// CPUQuota not cpu
		newResource := c.makeMemoryPriorSetting(newMemory, newCPU)
		updateConfig := enginecontainer.UpdateConfig{Resources: newResource}
		if err := reSetContainer(containerJSON.ID, node, updateConfig); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update container failed %v, %s", err, containerJSON.ID)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			// 如果是增加内存，失败的时候应该把内存还回去
			if memory > 0 {
				if err := c.store.UpdateNodeMem(podname, node.Name, memory, "+"); err != nil {
					log.Errorf("[doUpdateContainerWithMemoryPrior] failed to set mem back %s", containerJSON.ID)
				}
			}
			continue
		}
		// 如果是要降低内存，当执行成功的时候需要把内存还回去
		if memory < 0 {
			if err := c.store.UpdateNodeMem(podname, node.Name, -memory, "+"); err != nil {
				log.Errorf("[doUpdateContainerWithMemoryPrior] failed to set mem back %s", containerJSON.ID)
			}
		}

		container.Memory = newMemory
		if err := c.store.AddContainer(container); err != nil {
			log.Errorf("[doUpdateContainerWithMemoryPrior] update container meta failed %v", err)
			// 立即中断
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			return
		}
		ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: true}
	}
}

func (c *calcium) reallocNodesCPU(
	podname string,
	nodesInfoMap CPUNodeContainers,
) (CPUNodeContainersMap, error) {

	lock, err := c.Lock(podname, c.config.LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	// TODO too slow
	nodesCPUMap := CPUNodeContainersMap{}
	for cpu, nodesInfo := range nodesInfoMap {
		for node, containers := range nodesInfo {
			for _, container := range containers {
				// 把 CPU 还回去，变成新的可用资源
				// 即便有并发操作，不影响 Create 操作
				// 最坏情况就是 CPU 重叠了，可以外部纠正
				if err := c.store.UpdateNodeCPU(podname, node.Name, container.CPU, "+"); err != nil {
					return nil, err
				}
				node.CPU.Add(container.CPU)
			}

			// 按照 Node one by one 重新计算可以部署多少容器
			containersNum := len(containers)
			nodesInfo := []types.NodeInfo{
				types.NodeInfo{
					CPUAndMem: types.CPUAndMem{
						CpuMap: node.CPU,
						MemCap: 0,
					},
					Name: node.Name,
				},
			}

			result, changed, err := c.scheduler.SelectCPUNodes(nodesInfo, cpu, containersNum)
			if err != nil {
				for _, container := range containers {
					if err := c.store.UpdateNodeCPU(podname, node.Name, container.CPU, "-"); err != nil {
						return nil, err
					}
				}
				return nil, err
			}

			nodeCPUMap, isChanged := changed[node.Name]
			containersCPUMap, hasResult := result[node.Name]
			if isChanged && hasResult {
				node.CPU = nodeCPUMap
				if err := c.store.UpdateNode(node); err != nil {
					return nil, err
				}
				if _, ok := nodesCPUMap[cpu]; !ok {
					nodesCPUMap[cpu] = NodeCPUMap{}
				}
				nodesCPUMap[cpu][node] = containersCPUMap
			}
		}
	}
	return nodesCPUMap, nil
}

// mem not used in this prior
func (c *calcium) reallocContainersWithCPUPrior(
	ch chan *types.ReallocResourceMessage,
	pod *types.Pod,
	nodesInfoMap CPUNodeContainers,
	cpu float64, memory int64) {

	nodesCPUMap, err := c.reallocNodesCPU(pod.Name, nodesInfoMap)
	if err != nil {
		log.Errorf("[reallocContainersWithCPUPrior] realloc cpu resource failed %v", err)
		return
	}

	// 不并发操作了
	for cpu, nodesCPUResult := range nodesCPUMap {
		c.doReallocContainersWithCPUPrior(ch, pod.Name, nodesCPUResult, nodesInfoMap[cpu])
	}
}

func (c *calcium) doReallocContainersWithCPUPrior(
	ch chan *types.ReallocResourceMessage,
	podname string,
	nodesCPUResult NodeCPUMap,
	nodesInfoMap NodeContainers,
) {

	for node, cpuset := range nodesCPUResult {
		containers := nodesInfoMap[node]
		for index, container := range containers {
			//TODO 如果需要限制内存，需要在这里 inspect 一下
			quota := cpuset[index]
			resource := c.makeCPUPriorSetting(quota)
			updateConfig := enginecontainer.UpdateConfig{Resources: resource}
			if err := reSetContainer(container.ID, node, updateConfig); err != nil {
				log.Errorf("[doReallocContainersWithCPUPrior] update container failed %v", err)
				// TODO 这里理论上是可以恢复 CPU 占用表的，一来我们知道新的占用是怎样，二来我们也晓得老的占用是啥样
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			}

			container.CPU = quota
			if err := c.store.AddContainer(container); err != nil {
				log.Errorf("[doReallocContainersWithCPUPrior] update container meta failed %v", err)
				// 立即中断
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
				return
			}
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
		}
	}
}

func reSetContainer(ID string, node *types.Node, config enginecontainer.UpdateConfig) error {
	_, err := node.Engine.ContainerUpdate(context.Background(), ID, config)
	return err
}
