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

func (c *calcium) ReAllocResource(ids []string, cpu float64, mem int64) (chan *types.ReAllocResourceMessage, error) {
	containers, err := c.store.GetContainers(ids)
	if err != nil {
		return nil, err
	}

	meta := map[string]map[string][]*types.Container{}
	for _, container := range containers {
		meta[container.Podname][container.Nodename] = append(meta[container.Podname][container.Nodename], container)
	}

	ch := make(chan *types.ReAllocResourceMessage)
	for podname, containers := range meta {
		pod, err := c.store.GetPod(podname)
		if err != nil {
			return ch, err
		}
		if pod.Scheduler == CPU_SCHEDULER {
			go c.updateContainersWithCPUPrior(ch, podname, containers, cpu, mem)
			continue
		}
		go c.updateContainerWithMemoryPrior(ch, podname, containers, cpu, mem)
	}
	return ch, nil
}

func (c *calcium) checkMemoryResource(podname, nodename string, mem int64, count int) (*types.Node, error) {
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

func (c *calcium) updateContainerWithMemoryPrior(
	ch chan *types.ReAllocResourceMessage,
	podname string,
	containers map[string][]*types.Container, cpu float64, mem int64) {

	for nodename, containerList := range containers {
		node, err := c.checkMemoryResource(podname, nodename, mem, len(containerList))
		if err != nil {
			log.Errorf("[realloc] get node failed %v", err)
			continue
		}

		for _, container := range containerList {
			containerJSON, err := container.Inspect()
			if err != nil {
				log.Errorf("[realloc] get container failed %v", err)
				ch <- &types.ReAllocResourceMessage{ContainerID: containerJSON.ID, Success: false}
				continue
			}
			cpuQuota := int64(cpu * float64(utils.CpuPeriodBase))
			newCPUQuota := containerJSON.HostConfig.CPUQuota + cpuQuota
			newMemory := containerJSON.HostConfig.Memory + mem
			if newCPUQuota <= 0 || newMemory <= 0 {
				log.Warnf("[relloc] new resource invaild %s, %d, %d", containerJSON.ID, newCPUQuota, newMemory)
				ch <- &types.ReAllocResourceMessage{ContainerID: containerJSON.ID, Success: false}
				continue
			}

			// TODO Async
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
				ch <- &types.ReAllocResourceMessage{ContainerID: containerJSON.ID, Success: false}
				// 如果是增加内存，失败的时候应该把内存还回去
				if mem > 0 {
					if err := c.store.UpdateNodeMem(podname, nodename, mem, "+"); err != nil {
						log.Errorf("[realloc] failed to set mem back %s", containerJSON.ID)
					}
				}
			}
			// 如果是要降低内存，当执行成功的时候需要把内存还回去
			if mem < 0 {
				if err := c.store.UpdateNodeMem(podname, nodename, -mem, "+"); err != nil {
					log.Errorf("[realloc] failed to set mem back %s", containerJSON.ID)
				}
			}
			ch <- &types.ReAllocResourceMessage{ContainerID: containerJSON.ID, Success: true}
		}
	}
}

func (c *calcium) updateContainersWithCPUPrior(
	ch chan *types.ReAllocResourceMessage,
	podname string,
	containers map[string][]*types.Container, cpu float64, mem int64) {
}

func reSetContainer(ID string, node *types.Node, config enginecontainer.UpdateConfig, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	_, err := node.Engine.ContainerUpdate(ctx, ID, config)
	return err
}
