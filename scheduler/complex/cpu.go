/*
CPU 分配的核心算法
*/

package complexscheduler

import (
	"math"
	"sort"

	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cpuPriorPlan(cpu float64, memory int64, scheduleInfos []resourcetypes.ScheduleInfo, maxShareCore, coreShare int) ([]resourcetypes.ScheduleInfo, map[string][]types.CPUMap, int, error) {
	var nodeWorkload = map[string][]types.CPUMap{}
	volTotal := 0

	for p, scheduleInfo := range scheduleInfos {
		// 统计全局 CPU，为非 numa 或者跨 numa 计算
		globalCPUMap := scheduleInfo.CPU
		// 统计全局 Memory
		globalMemCap := scheduleInfo.MemCap
		// 计算每个 numa node 的分配策略
		// 得到 numa CPU 分组
		numaCPUMap := map[string]types.CPUMap{}
		for cpuID, nodeID := range scheduleInfo.NUMA {
			if _, ok := numaCPUMap[nodeID]; !ok {
				numaCPUMap[nodeID] = types.CPUMap{}
			}
			cpuCount, ok := scheduleInfo.CPU[cpuID]
			if !ok {
				continue
			}
			numaCPUMap[nodeID][cpuID] = cpuCount
		}
		for nodeID, nodeCPUMap := range numaCPUMap {
			nodeMemCap, ok := scheduleInfo.NUMAMemory[nodeID]
			if !ok {
				continue
			}
			cap, plan := calculateCPUPlan(nodeCPUMap, nodeMemCap, cpu, memory, maxShareCore, coreShare)
			if cap > 0 {
				if _, ok := nodeWorkload[scheduleInfo.Name]; !ok {
					nodeWorkload[scheduleInfo.Name] = []types.CPUMap{}
				}
				volTotal += updateScheduleInfoCapacity(&scheduleInfos[p], cap)
				globalMemCap -= int64(cap) * memory
				for _, cpuPlan := range plan {
					globalCPUMap.Sub(cpuPlan)
					nodeWorkload[scheduleInfo.Name] = append(nodeWorkload[scheduleInfo.Name], cpuPlan)
				}
			}
			log.Infof("[cpuPriorPlan] node %s numa node %s deploy capacity %d", scheduleInfo.Name, nodeID, cap)
		}
		// 非 numa
		// 或者是扣掉 numa 分配后剩下的资源里面
		cap, plan := calculateCPUPlan(globalCPUMap, globalMemCap, cpu, memory, maxShareCore, coreShare)
		if cap > 0 {
			if _, ok := nodeWorkload[scheduleInfo.Name]; !ok {
				nodeWorkload[scheduleInfo.Name] = []types.CPUMap{}
			}
			scheduleInfos[p].Capacity += cap
			volTotal += cap
			nodeWorkload[scheduleInfo.Name] = append(nodeWorkload[scheduleInfo.Name], plan...)
		}
		log.Infof("[cpuPriorPlan] node %s total deploy capacity %d", scheduleInfo.Name, scheduleInfos[p].Capacity)
	}

	// 裁剪掉不能部署的
	sort.Slice(scheduleInfos, func(i, j int) bool { return scheduleInfos[i].Capacity < scheduleInfos[j].Capacity })
	p := sort.Search(len(scheduleInfos), func(i int) bool { return scheduleInfos[i].Capacity > 0 })
	if p == len(scheduleInfos) {
		return nil, nil, 0, types.ErrInsufficientRes
	}

	return scheduleInfos[p:], nodeWorkload, volTotal, nil
}

func calculateCPUPlan(CPUMap types.CPUMap, MemCap int64, cpu float64, memory int64, maxShareCore, coreShare int) (int, []types.CPUMap) {
	host := newHost(CPUMap, coreShare)
	plan := host.distributeOneRation(cpu, maxShareCore)
	memLimit := math.MaxInt64
	if memory != 0 {
		memLimit = int(MemCap / memory)
	}
	cap := len(plan) // 每个node可以放的容器数
	if cap > memLimit {
		plan = plan[:memLimit]
		cap = memLimit
	}
	if cap <= 0 {
		plan = nil
	}
	return cap, plan
}
