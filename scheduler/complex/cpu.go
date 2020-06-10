/*
CPU 分配的核心算法
*/

package complexscheduler

import (
	"math"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cpuPriorPlan(cpu float64, memory int64, nodesInfo []types.NodeInfo, maxShareCore, coreShare int) ([]types.NodeInfo, map[string][]types.CPUMap, int, error) {
	var nodeContainer = map[string][]types.CPUMap{}
	volTotal := 0

	for p, nodeInfo := range nodesInfo {
		// 统计全局 CPU，为非 numa 或者跨 numa 计算
		globalCPUMap := nodeInfo.CPUMap
		// 统计全局 Memory
		globalMemCap := nodeInfo.MemCap
		// 计算每个 numa node 的分配策略
		// 得到 numa CPU 分组
		numaCPUMap := map[string]types.CPUMap{}
		for cpuID, nodeID := range nodeInfo.NUMA {
			if _, ok := numaCPUMap[nodeID]; !ok {
				numaCPUMap[nodeID] = types.CPUMap{}
			}
			cpuCount, ok := nodeInfo.CPUMap[cpuID]
			if !ok {
				continue
			}
			numaCPUMap[nodeID][cpuID] = cpuCount
		}
		for nodeID, nodeCPUMap := range numaCPUMap {
			nodeMemCap, ok := nodeInfo.NUMAMemory[nodeID]
			if !ok {
				continue
			}
			cap, plan := calculateCPUPlan(nodeCPUMap, nodeMemCap, cpu, memory, maxShareCore, coreShare)
			if cap > 0 {
				if _, ok := nodeContainer[nodeInfo.Name]; !ok {
					nodeContainer[nodeInfo.Name] = []types.CPUMap{}
				}
				volTotal += updateNodeInfoCapacity(&nodesInfo[p], cap)
				globalMemCap -= int64(cap) * memory
				for _, cpuPlan := range plan {
					globalCPUMap.Sub(cpuPlan)
					nodeContainer[nodeInfo.Name] = append(nodeContainer[nodeInfo.Name], cpuPlan)
				}
			}
			log.Infof("[cpuPriorPlan] node %s numa node %s deploy capacity %d", nodeInfo.Name, nodeID, cap)
		}
		// 非 numa
		// 或者是扣掉 numa 分配后剩下的资源里面
		cap, plan := calculateCPUPlan(globalCPUMap, globalMemCap, cpu, memory, maxShareCore, coreShare)
		if cap > 0 {
			if _, ok := nodeContainer[nodeInfo.Name]; !ok {
				nodeContainer[nodeInfo.Name] = []types.CPUMap{}
			}
			nodesInfo[p].Capacity += cap
			volTotal += cap
			nodeContainer[nodeInfo.Name] = append(nodeContainer[nodeInfo.Name], plan...)
		}
		log.Infof("[cpuPriorPlan] node %s total deploy capacity %d", nodeInfo.Name, nodesInfo[p].Capacity)
	}

	// 裁剪掉不能部署的
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Capacity < nodesInfo[j].Capacity })
	p := sort.Search(len(nodesInfo), func(i int) bool { return nodesInfo[i].Capacity > 0 })
	if p == len(nodesInfo) {
		return nil, nil, 0, types.ErrInsufficientRes
	}

	return nodesInfo[p:], nodeContainer, volTotal, nil
}

func calculateCPUPlan(CPUMap types.CPUMap, MemCap int64, cpu float64, memory int64, maxShareCore, coreShare int) (int, []types.CPUMap) {
	host := newHost(CPUMap, coreShare)
	plan := host.distributeOneRation(cpu, maxShareCore)
	memLimit := math.MaxInt16
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
