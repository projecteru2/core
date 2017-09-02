package calcium

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
)

func TestReallocWithCPUPrior(t *testing.T) {
	initMockConfig()
	containersInfo := map[*types.Pod]NodeContainers{}
	pod, _ := mockc.GetPod("pod1")
	containersInfo[pod] = NodeContainers{}
	node, _ := mockc.GetNode(pod.Name, updatenodename)
	cpuContainersInfo := map[*types.Pod]CPUNodeContainers{}
	cpuContainersInfo[pod] = CPUNodeContainers{}

	containers, _ := mockc.GetContainers(ToUpdateContainerIDs)
	for _, container := range containers {
		containersInfo[pod][node] = append(containersInfo[pod][node], container)
	}

	// 扩容
	cpu := 0.1
	CPURate := mockc.calculateCPUUsage(containers[0])
	newCPURequire := CPURate + cpu
	cpuContainersInfo[pod][newCPURequire] = NodeContainers{}
	cpuContainersInfo[pod][newCPURequire][node] = []*types.Container{}
	for _, container := range containers {
		cpuContainersInfo[pod][newCPURequire][node] = append(cpuContainersInfo[pod][newCPURequire][node], container)
	}

	ch1 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch1)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		for pod, _ := range containersInfo {
			nodeCPUContainersInfo := cpuContainersInfo[pod]
			go func(pod *types.Pod, nodeCPUContainersInfo CPUNodeContainers) {
				defer wg.Done()
				mockc.reallocContainersWithCPUPrior(ch1, pod, nodeCPUContainersInfo, cpu, 0)
			}(pod, nodeCPUContainersInfo)
		}
		wg.Wait()
	}()
	for msg := range ch1 {
		assert.True(t, msg.Success)
	}

	// 缩容
	cpu = -0.1
	newCPURequire = CPURate + cpu
	cpuContainersInfo[pod][newCPURequire] = NodeContainers{}
	cpuContainersInfo[pod][newCPURequire][node] = containers
	ch2 := make(chan *types.ReallocResourceMessage)
	go func() {
		defer close(ch2)
		wg := sync.WaitGroup{}
		wg.Add(len(containersInfo))
		go func(pod *types.Pod, nodeCPUContainersInfo CPUNodeContainers) {
			defer wg.Done()
			mockc.reallocContainersWithCPUPrior(ch2, pod, nodeCPUContainersInfo, cpu, 0)
		}(pod, cpuContainersInfo[pod])

	}()
	for msg := range ch2 {
		assert.True(t, msg.Success)
	}

}

func TestReallocWithMemoryPrior(t *testing.T) {
	initMockConfig()
	containersInfo := map[*types.Pod]NodeContainers{}
	pod, _ := mockc.GetPod("pod1")
	containersInfo[pod] = NodeContainers{}
	node, _ := mockc.GetNode(pod.Name, updatenodename)

	containers, _ := mockc.GetContainers(ToUpdateContainerIDs)
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
				mockc.reallocContainerWithMemoryPrior(ch1, pod, nodeContainers, 0.2, 100000)
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
				mockc.reallocContainerWithMemoryPrior(ch2, pod, nodeContainers, -0.2, -100000)
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
				mockc.reallocContainerWithMemoryPrior(ch3, pod, nodeContainers, 0, 2600000000)
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
				mockc.reallocContainerWithMemoryPrior(ch4, pod, nodeContainers, 0, -268400000)
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

	ids := ToUpdateContainerIDs
	cpuadd := float64(0.1)
	memadd := int64(1)

	ch, err := mockc.ReallocResource(ids, cpuadd, memadd)
	assert.Nil(t, err)

	for msg := range ch {
		assert.True(t, msg.Success)
	}

	clnt := mockDockerClient()
	for _, id := range ids {
		CJ, _ := clnt.ContainerInspect(context.Background(), id)

		// diff memory
		newMem := CJ.HostConfig.Resources.Memory
		assert.Equal(t, newMem-memadd, appmemory)

		// diff CPU
		newCPU := CJ.HostConfig.Resources.CPUQuota
		diff := float64(newCPU) - cpuadd*utils.CpuPeriodBase
		assert.Equal(t, int(diff), utils.CpuPeriodBase)
	}
}
