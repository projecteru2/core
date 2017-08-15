package calcium

import (
	"context"
	"fmt"
	"time"

	log "github.com/Sirupsen/logrus"
	enginecontainer "github.com/docker/docker/api/types/container"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
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
		containersInfo[pod][node] = append(containersInfo[pod][node], container)
		if pod.Scheduler == CPU_PRIOR {
			if _, ok := cpuContainersInfo[pod]; !ok {
				cpuContainersInfo[pod] = CPUNodeContainers{}
			}
			podCPUContainersInfo := cpuContainersInfo[pod]
			newCPURequire := c.calculateCPUUsage(container) + cpu
			if newCPURequire < 0.0 {
				return nil, fmt.Errorf("[realloc] cpu can not below zero")
			}
			if _, ok := podCPUContainersInfo[newCPURequire]; !ok {
				podCPUContainersInfo[newCPURequire][node] = []*types.Container{}
			}
			podCPUContainersInfo[newCPURequire][node] = append(podCPUContainersInfo[newCPURequire][node], container)
		}
	}

	ch := make(chan *types.ReallocResourceMessage)
	for pod, nodeContainers := range containersInfo {
		if pod.Scheduler == CPU_PRIOR {
			nodeCPUContainersInfo := cpuContainersInfo[pod]
			go c.reallocContainersWithCPUPrior(ch, pod, nodeCPUContainersInfo, cpu, mem)
			continue
		}
		go c.reallocContainerWithMemoryPrior(ch, pod, nodeContainers, cpu, mem)
	}
	return ch, nil
}

func (c *calcium) reallocNodesMemory(podname string, nodeContainers NodeContainers, memory int64) error {
	lock, err := c.Lock(podname, 30)
	if err != nil {
		return err
	}
	defer lock.Unlock()
	for node, containers := range nodeContainers {
		if memory > 0 {
			if cap := int(node.MemCap / memory); cap < len(containers) {
				return fmt.Errorf("[realloc] Not enough resource %s", node.Name)
			}
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

	if err := c.reallocNodesMemory(pod.Name, nodeContainers, memory); err != nil {
		log.Errorf("[realloc] realloc memory failed %v", err)
		return
	}

	for node, containers := range nodeContainers {
		go c.doUpdateContainerWithMemoryPrior(ch, pod.Name, node, containers, cpu, memory)
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
			log.Errorf("[realloc] get container failed %v", err)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}

		cpuQuota := int64(cpu * float64(utils.CpuPeriodBase))
		newCPUQuota := containerJSON.HostConfig.CPUQuota + cpuQuota
		newMemory := containerJSON.HostConfig.Memory + memory
		if newCPUQuota <= 0 || newMemory <= 0 {
			log.Warnf("[relloc] new resource invaild %s, %d, %d", containerJSON.ID, newCPUQuota, newMemory)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}

		// CPUQuota not cpu
		newResource := c.makeMemoryPriorSetting(newMemory, float64(newCPUQuota)/float64(utils.CpuPeriodBase))
		updateConfig := enginecontainer.UpdateConfig{Resources: newResource}
		if err := reSetContainer(containerJSON.ID, node, updateConfig, c.config.Timeout.Realloc); err != nil {
			log.Errorf("[realloc] update container failed %v, %s", err, containerJSON.ID)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			// 如果是增加内存，失败的时候应该把内存还回去
			if memory > 0 {
				if err := c.store.UpdateNodeMem(podname, node.Name, memory, "+"); err != nil {
					log.Errorf("[realloc] failed to set mem back %s", containerJSON.ID)
				}
			}
			continue
		}
		// 如果是要降低内存，当执行成功的时候需要把内存还回去
		if memory < 0 {
			if err := c.store.UpdateNodeMem(podname, node.Name, -memory, "+"); err != nil {
				log.Errorf("[realloc] failed to set mem back %s", containerJSON.ID)
			}
		}
		ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: true}
	}
}

func (c *calcium) reallocNodesCPU(
	podname string,
	nodesInfoMap CPUNodeContainers,
) (CPUNodeContainersMap, error) {

	lock, err := c.Lock(podname, 30)
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
				return nil, err
			}

			nodeCPUMap, isChanged := changed[node.Name]
			containersCPUMap, hasResult := result[node.Name]
			if isChanged && hasResult {
				node.CPU = nodeCPUMap
				if err := c.store.UpdateNode(node); err != nil {
					return nil, err
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
		log.Errorf("[realloc] realloc cpu resource failed %v", err)
		return
	}
	for cpu, nodesCPUResult := range nodesCPUMap {
		go c.doReallocContainersWithCPUPrior(ch, pod.Name, nodesCPUResult, nodesInfoMap[cpu])
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
			resource := c.makeCPUPriorSetting(cpuset[index])
			updateConfig := enginecontainer.UpdateConfig{Resources: resource}
			if err := reSetContainer(container.ID, node, updateConfig, c.config.Timeout.Realloc); err != nil {
				log.Errorf("[realloc] update container failed %v", err)
				// TODO 这里理论上是可以恢复 CPU 占用表的，一来我们知道新的占用是怎样，二来我们也晓得老的占用是啥样
				ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: false}
			}
			ch <- &types.ReallocResourceMessage{ContainerID: container.ID, Success: true}
		}
	}
}

func reSetContainer(ID string, node *types.Node, config enginecontainer.UpdateConfig, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := node.Engine.ContainerUpdate(ctx, ID, config)
	return err
}
