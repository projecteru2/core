package calcium

import (
	"context"
	"sync"
	"testing"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestReallocWithCPUPrior(t *testing.T) {
	initMockConfig()
	containersInfo := map[*types.Pod]NodeContainers{}
	ctx := context.Background()
	pod, _ := mockc.GetPod(ctx, "pod1")
	containersInfo[pod] = NodeContainers{}
	node, _ := mockc.GetNode(ctx, pod.Name, updatenodename)
	cpuContainersInfo := map[*types.Pod]CPUMemNodeContainers{}
	cpuContainersInfo[pod] = CPUMemNodeContainers{}

	containers, _ := mockc.GetContainers(ctx, ToUpdateContainerIDs)
	for _, container := range containers {
		containersInfo[pod][node] = append(containersInfo[pod][node], container)
	}

	// 扩容
	cpu := 0.1
	mem := int64(1)
	CPURate := calculateCPUUsage(mockc.config.Scheduler.ShareBase, containers[0])
	newCPURequire := CPURate + cpu
	newMEMRequire := containers[0].Memory + mem
	cpuContainersInfo[pod][newCPURequire] = map[int64]NodeContainers{}
	cpuContainersInfo[pod][newCPURequire][newMEMRequire] = NodeContainers{}
	cpuContainersInfo[pod][newCPURequire][newMEMRequire][node] = []*types.Container{}
	for _, container := range containers {
		cpuContainersInfo[pod][newCPURequire][newMEMRequire][node] = append(cpuContainersInfo[pod][newCPURequire][mem][node], container)
	}

	ch1 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch1)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod := range containersInfo {
			cpuMemNodeContainersInfo := cpuContainersInfo[pod]
			go func(pod *types.Pod, cpuMemNodeContainersInfo CPUMemNodeContainers) {
				defer wg.Done()
				mockc.reallocContainersWithCPUPrior(ctx, ch1, pod, cpuMemNodeContainersInfo)
			}(pod, cpuMemNodeContainersInfo)
		}
		wg.Wait()
	}()
	for msg := range ch1 {
		assert.True(t, msg.Success)
	}

	// 缩容
	cpu = -0.1
	mem = int64(-1)
	newCPURequire = CPURate + cpu
	newMEMRequire = containers[0].Memory + mem
	cpuContainersInfo[pod][newCPURequire] = map[int64]NodeContainers{}
	cpuContainersInfo[pod][newCPURequire][newMEMRequire] = NodeContainers{}
	cpuContainersInfo[pod][newCPURequire][newMEMRequire][node] = []*types.Container{}
	ch2 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch2)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod := range containersInfo {
			cpuMemNodeContainersInfo := cpuContainersInfo[pod]
			go func(pod *types.Pod, cpuMemNodeContainersInfo CPUMemNodeContainers) {
				defer wg.Done()
				mockc.reallocContainersWithCPUPrior(ctx, ch2, pod, cpuMemNodeContainersInfo)
			}(pod, cpuMemNodeContainersInfo)
		}
		wg.Wait()
	}()
	for msg := range ch2 {
		assert.True(t, msg.Success)
	}

}

func TestReallocWithMemoryPrior(t *testing.T) {
	initMockConfig()
	containersInfo := map[*types.Pod]NodeContainers{}
	ctx := context.Background()
	pod, _ := mockc.GetPod(ctx, "pod1")
	containersInfo[pod] = NodeContainers{}
	node, _ := mockc.GetNode(ctx, pod.Name, updatenodename)

	containers, _ := mockc.GetContainers(ctx, ToUpdateContainerIDs)
	for _, container := range containers {
		containersInfo[pod][node] = append(containersInfo[pod][node], container)
	}

	ch1 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch1)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod, nodeContainers := range containersInfo {
			go func(pod *types.Pod, nodeContainers NodeContainers) {
				defer wg.Done()
				mockc.reallocContainerWithMemoryPrior(ctx, ch1, pod, nodeContainers, 0.2, 100000)
			}(pod, nodeContainers)
		}
		wg.Wait()
	}()

	for msg := range ch1 {
		assert.True(t, msg.Success)
	}

	ch2 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch2)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod, nodeContainers := range containersInfo {
			go func(pod *types.Pod, nodeContainers NodeContainers) {
				defer wg.Done()
				mockc.reallocContainerWithMemoryPrior(ctx, ch2, pod, nodeContainers, -0.2, -100000)
			}(pod, nodeContainers)
		}
		wg.Wait()
	}()

	for msg := range ch2 {
		assert.True(t, msg.Success)
	}

	ch3 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch3)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod, nodeContainers := range containersInfo {
			go func(pod *types.Pod, nodeContainers NodeContainers) {
				defer wg.Done()
				mockc.reallocContainerWithMemoryPrior(ctx, ch3, pod, nodeContainers, 0, 2600000000)
			}(pod, nodeContainers)
		}
		wg.Wait()
	}()

	for msg := range ch3 {
		assert.False(t, msg.Success)
	}

	ch4 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch4)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod, nodeContainers := range containersInfo {
			go func(pod *types.Pod, nodeContainers NodeContainers) {
				defer wg.Done()
				mockc.reallocContainerWithMemoryPrior(ctx, ch4, pod, nodeContainers, 0, -268400000)
			}(pod, nodeContainers)
		}
		wg.Wait()
	}()

	for msg := range ch4 {
		assert.False(t, msg.Success)
	}

}

func TestReallocResource(t *testing.T) {
	initMockConfig()

	IDs := ToUpdateContainerIDs
	cpuadd := float64(0.1)
	memadd := int64(1)
	ctx := context.Background()
	ch, err := mockc.ReallocResource(ctx, IDs, cpuadd, memadd)
	assert.Nil(t, err)

	for msg := range ch {
		assert.True(t, msg.Success)
	}

	clnt := mockDockerClient()
	for _, ID := range IDs {
		CJ, _ := clnt.ContainerInspect(ctx, ID)

		// diff memory
		newMem := CJ.HostConfig.Resources.Memory
		assert.Equal(t, newMem-memadd, appmemory)

		// diff CPU
		newCPU := CJ.HostConfig.Resources.CPUQuota
		diff := float64(newCPU) - cpuadd*cluster.CPUPeriodBase
		assert.Equal(t, int(diff), cluster.CPUPeriodBase)
	}
}

func calculateCPUUsage(shareBase int64, container *types.Container) float64 {
	var full, fragment int64
	for _, usage := range container.CPU {
		if usage == shareBase {
			full++
			continue
		}
		fragment += usage
	}
	return float64(full) + float64(fragment/shareBase)
}
