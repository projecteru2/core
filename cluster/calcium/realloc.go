package calcium

import (
	"context"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	enginecontainer "github.com/docker/docker/api/types/container"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

func (c *calcium) ReallocResource(ids []string, cpu float64, mem int64) (chan *types.ReallocResourceMessage, error) {
	containers, err := c.store.GetContainers(ids)
	if err != nil {
		return nil, err
	}

	meta := map[string]map[string][]*types.Container{}
	for _, container := range containers {
		meta[container.Podname][container.Nodename] = append(meta[container.Podname][container.Nodename], container)
	}

	ch := make(chan *types.ReallocResourceMessage)
	for podname, containers := range meta {
		pod, err := c.store.GetPod(podname)
		if err != nil {
			return ch, err
		}
		if pod.Scheduler == CPU_PRIOR {
			go c.reallocContainersWithCPUPrior(ch, podname, containers, cpu, mem)
			continue
		}
		go c.reallocContainerWithMemoryPrior(ch, podname, containers, cpu, mem)
	}
	return ch, nil
}

func (c *calcium) reallocNodeMemory(podname, nodename string, mem int64, count int) (*types.Node, error) {
	lock, err := c.Lock(podname, 30)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	node, err := c.GetNode(podname, nodename)
	if err != nil {
		return nil, err
	}

	if mem > 0 {
		if cap := int(node.MemCap / mem); cap < count {
			return nil, fmt.Errorf("Not enough resource %s", nodename)
		}
		// 预先扣除
		return node, c.store.UpdateNodeMem(podname, nodename, int64(count)*mem, "-")
	}
	return node, nil
}

func (c *calcium) reallocContainerWithMemoryPrior(
	ch chan *types.ReallocResourceMessage,
	podname string,
	nodeContainers map[string][]*types.Container, cpu float64, mem int64) {

	for nodename, containers := range nodeContainers {
		node, err := c.reallocNodeMemory(podname, nodename, mem, len(containers))
		if err != nil {
			log.Errorf("[realloc] get node failed %v", err)
			continue
		}
		go c.doUpdateContainerWithMemoryPrior(ch, podname, node, containers, cpu, mem)
	}
}

func (c *calcium) doUpdateContainerWithMemoryPrior(
	ch chan *types.ReallocResourceMessage,
	podname string,
	node *types.Node,
	containers []*types.Container,
	cpu float64, mem int64) {

	for _, container := range containers {
		containerJSON, err := container.Inspect()
		if err != nil {
			log.Errorf("[realloc] get container failed %v", err)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}

		cpuQuota := int64(cpu * float64(utils.CpuPeriodBase))
		newCPUQuota := containerJSON.HostConfig.CPUQuota + cpuQuota
		newMemory := containerJSON.HostConfig.Memory + mem
		if newCPUQuota <= 0 || newMemory <= 0 {
			log.Warnf("[relloc] new resource invaild %s, %d, %d", containerJSON.ID, newCPUQuota, newMemory)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			continue
		}

		// TODO config timeout
		newResource := enginecontainer.Resources{
			Memory:     newMemory,
			MemorySwap: newMemory,
			CPUPeriod:  utils.CpuPeriodBase,
			CPUQuota:   newCPUQuota,
		}
		updateConfig := enginecontainer.UpdateConfig{Resources: newResource}
		if err := reSetContainer(containerJSON.ID, node, updateConfig, 10*time.Second); err != nil {
			log.Errorf("[realloc] update container failed %v, %s", err, containerJSON.ID)
			ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: false}
			// 如果是增加内存，失败的时候应该把内存还回去
			if mem > 0 {
				if err := c.store.UpdateNodeMem(podname, node.Name, mem, "+"); err != nil {
					log.Errorf("[realloc] failed to set mem back %s", containerJSON.ID)
				}
			}
			continue
		}
		// 如果是要降低内存，当执行成功的时候需要把内存还回去
		if mem < 0 {
			if err := c.store.UpdateNodeMem(podname, node.Name, -mem, "+"); err != nil {
				log.Errorf("[realloc] failed to set mem back %s", containerJSON.ID)
			}
		}
		ch <- &types.ReallocResourceMessage{ContainerID: containerJSON.ID, Success: true}
	}
}

const CPUSHARE_BASE = 10

func calculateCPUUsage(container *types.Container) float64 {
	var full, fragment float64
	for _, usage := range container.CPU {
		if usage == CPUSHARE_BASE {
			full += 1.0
			continue
		}
		fragment += float64(usage)
	}
	return full + fragment/float64(CPUSHARE_BASE)
}

func (c *calcium) reallocNodesCPU(nodesInfoMap map[float64]map[string][]*types.Container, podname string) (map[float64]map[string][]types.CPUMap, error) {
	lock, err := c.Lock(podname, 30)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock()

	// TODO too slow
	nodesCPUMap := map[float64]map[string][]types.CPUMap{}
	for cpu, nodesInfo := range nodesInfoMap {
		for nodename, containers := range nodesInfo {
			for _, container := range containers {
				// 把 CPU 还回去，变成新的可用资源
				// 即便有并发操作，不影响 Create 操作
				// 最坏情况就是 CPU 重叠了，可以外部纠正
				if err := c.store.UpdateNodeCPU(podname, nodename, container.CPU, "+"); err != nil {
					return nil, err
				}
			}
			node, err := c.GetNode(podname, nodename)
			if err != nil {
				return nil, err
			}
			containersNum := len(containers)
			nodesInfo := []types.NodeInfo{
				types.NodeInfo{
					CPUAndMem: types.CPUAndMem{
						CpuMap: node.CPU,
						MemCap: 0,
					},
					Name: nodename,
				},
			}
			result, changed, err := c.scheduler.SelectCPUNodes(nodesInfo, cpu, containersNum)
			if err != nil {
				return nil, err
			}
			nodeCPUMap, isChanged := changed[nodename]
			containersCPUMap, hasResult := result[nodename]
			if isChanged && hasResult {
				node.CPU = nodeCPUMap
				if err := c.store.UpdateNode(node); err != nil {
					return nil, err
				}
				nodesCPUMap[cpu][nodename] = containersCPUMap
			}
		}
	}
	return nodesCPUMap, nil
}

// mem not used in this prior
func (c *calcium) reallocContainersWithCPUPrior(
	ch chan *types.ReallocResourceMessage,
	podname string,
	nodeContainers map[string][]*types.Container, cpu float64, mem int64) {

	nodesInfoMap := map[float64]map[string][]*types.Container{}
	for nodename, containers := range nodeContainers {
		for _, container := range containers {
			newCPU := calculateCPUUsage(container) + cpu
			if newCPU <= 0.0 {
				log.Warnf("[realloc] cpu set below zero")
				continue
			}
			if _, ok := nodesInfoMap[newCPU]; !ok {
				nodesInfoMap[newCPU] = map[string][]*types.Container{}
			}
			nodesInfoMap[newCPU][nodename] = append(nodesInfoMap[newCPU][nodename], container)
		}
	}
	nodesCPUMap, err := c.reallocNodesCPU(nodesInfoMap, podname)
	if err != nil {
		log.Errorf("[realloc] realloc cpu res failed %v", err)
		return
	}
	for cpu, nodesCPUResult := range nodesCPUMap {
		go c.doReallocContainersWithCPUPrior(ch, podname, nodesCPUResult, nodesInfoMap[cpu])
	}
}

func (c *calcium) doReallocContainersWithCPUPrior(
	ch chan *types.ReallocResourceMessage,
	podname string,
	nodesCPUResult map[string][]types.CPUMap,
	nodesInfoMap map[string][]*types.Container,
) {
	for nodename, cpuset := range nodesCPUResult {
		node, err := c.GetNode(podname, nodename)
		if err != nil {
			return
		}
		containers := nodesInfoMap[nodename]
		for index, container := range containers {
			// TODO dante 来重构吧，应该能抽出一个公共函数
			// TODO 同 create container #543
			// TODO 抽出 CPUShare 的配置
			var shareQuota int64 = 10
			labels := []string{}
			for label, share := range cpuset[index] {
				labels = append(labels, label)
				if share < shareQuota {
					shareQuota = share
				}
			}
			cpuShares := int64(float64(shareQuota) / float64(10) * float64(1024))
			cpuSetCpus := strings.Join(labels, ",")
			resource := enginecontainer.Resources{
				CPUShares:  cpuShares,
				CpusetCpus: cpuSetCpus,
			}

			updateConfig := enginecontainer.UpdateConfig{Resources: resource}
			if err := reSetContainer(container.ID, node, updateConfig, 10*time.Second); err != nil {
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
