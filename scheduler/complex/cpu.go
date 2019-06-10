/*
CPU 分配的核心算法
*/

package complexscheduler

import (
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

type cpuInfo struct {
	no     string
	pieces int
}

type host struct {
	full     []cpuInfo
	fragment []cpuInfo
	share    int
}

func newHost(cpusMap types.CPUMap, share int) *host {
	result := &host{
		share:    share,
		full:     []cpuInfo{},
		fragment: []cpuInfo{},
	}
	for no, pieces := range cpusMap {
		// 整数核不应该切分
		if pieces >= share && pieces%share == 0 {
			// 只给 share 份
			result.full = append(result.full, cpuInfo{no: no, pieces: pieces})
		} else {
			result.fragment = append(result.fragment, cpuInfo{no: no, pieces: pieces})
		}
	}
	// 确保优先分配更碎片的核
	sort.Slice(result.fragment, func(i, j int) bool { return result.fragment[i].pieces < result.fragment[j].pieces })
	// 确保优先分配负重更大的整数核
	sort.Slice(result.full, func(i, j int) bool { return result.full[i].pieces < result.full[j].pieces })

	return result
}

func (h *host) getComplexResult(full, fragment, maxShareCore int) []types.CPUMap {
	if maxShareCore == -1 {
		maxShareCore = len(h.full) - full // 减枝，M == N 的情况下预留至少一个 full 量的核数
	} else {
		maxShareCore -= len(h.fragment)
	}

	// 计算默认情况下能部署多少个
	fragmentResultBase := h.getFragmentResult(fragment, h.fragment)
	fullResultBase := h.getFullResult(full, h.full)
	fragmentResultCount := len(fragmentResultBase)
	fullResultCount := len(fullResultBase)

	baseLine := min(fragmentResultCount, fullResultCount)
	fragmentResult := fragmentResultBase
	fullResult := fullResultBase
	for i := 1; i < maxShareCore+1; i++ {
		fragmentResultBase = h.getFragmentResult(fragment, append(h.fragment, h.full[:i]...))
		fullResultBase = h.getFullResult(full, h.full[i:])
		fragmentResultCount = len(fragmentResultBase)
		fullResultCount = len(fullResultBase)

		canDeployNum := min(fragmentResultCount, fullResultCount)
		if canDeployNum > baseLine {
			baseLine = canDeployNum
			fragmentResult = fragmentResultBase
			fullResult = fullResultBase
		}
	}

	result := []types.CPUMap{}
	for i := 0; i < baseLine; i++ {
		r := types.CPUMap{}
		for no, pieces := range fullResult[i] {
			if _, ok := r[no]; ok {
				r[no] += pieces
			} else {
				r[no] = pieces
			}
		}
		for no, pieces := range fragmentResult[i] {
			r[no] = pieces
		}
		result = append(result, r)
	}

	return result
}

func (h *host) getContainerCores(cpu float64, maxShareCore int) []types.CPUMap {
	cpu = cpu * float64(h.share)
	fullRequire := int(cpu) / h.share
	fragmentRequire := int(cpu) % h.share

	if fullRequire == 0 {
		if maxShareCore == -1 {
			// 这个时候就把所有的核都当成碎片核
			maxShareCore = len(h.full) + len(h.fragment)
		}
		diff := maxShareCore - len(h.fragment)
		h.fragment = append(h.fragment, h.full[:diff]...)

		return h.getFragmentResult(fragmentRequire, h.fragment)
	}

	if fragmentRequire == 0 {
		return h.getFullResult(fullRequire, h.full)
	}

	return h.getComplexResult(fullRequire, fragmentRequire, maxShareCore)
}

func (h *host) getFragmentResult(fragment int, cpus []cpuInfo) []types.CPUMap {
	result := []types.CPUMap{}

	for i := range cpus {
		count := cpus[i].pieces / fragment
		for j := 0; j < count; j++ {
			result = append(result, types.CPUMap{cpus[i].no: fragment})
		}
	}
	return result
}

func (h *host) getFullResult(full int, cpus []cpuInfo) []types.CPUMap {
	result := []types.CPUMap{}
	count := len(cpus) / full
	newCpus := []cpuInfo{}
	for i := 0; i < count; i++ {
		plan := types.CPUMap{}
		for j := i * full; j < i*full+full; j++ {
			// 洗掉没配额的 CPU
			last := cpus[j].pieces - h.share
			if last > 0 {
				newCpus = append(newCpus, cpuInfo{cpus[j].no, last})
			}
			plan[cpus[j].no] = h.share
		}
		result = append(result, plan)
	}

	if len(newCpus)/full > 0 {
		return append(result, h.getFullResult(full, newCpus)...)
	}
	return result
}

func cpuPriorPlan(cpu float64, memory int64, nodesInfo []types.NodeInfo, maxShareCore, coreShare int) ([]types.NodeInfo, map[string][]types.CPUMap, int, error) {
	var nodeContainer = map[string][]types.CPUMap{}
	volTotal := 0

	for p, nodeInfo := range nodesInfo {
		// 统计全局 CPU，为非 numa 或者跨 numa 计算
		globalCPUMap := nodeInfo.CPUMap
		// 统计全局 MEM
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
			nodeMemCap, ok := nodeInfo.NUMAMem[nodeID]
			if !ok {
				continue
			}
			cap, plan := calcuateCPUPlan(nodeCPUMap, nodeMemCap, cpu, memory, maxShareCore, coreShare)
			if cap > 0 {
				if _, ok := nodeContainer[nodeInfo.Name]; !ok {
					nodeContainer[nodeInfo.Name] = []types.CPUMap{}
				}
				nodesInfo[p].Capacity += cap
				volTotal += cap
				globalMemCap -= int64(cap) * memory
				for _, cpuPlan := range plan {
					globalCPUMap.Sub(cpuPlan)
					nodeContainer[nodeInfo.Name] = append(nodeContainer[nodeInfo.Name], cpuPlan)
				}
			}
		}
		// 非 numa
		// 或者是扣掉 numa 分配后剩下的资源里面
		cap, plan := calcuateCPUPlan(globalCPUMap, globalMemCap, cpu, memory, maxShareCore, coreShare)
		if cap > 0 {
			if _, ok := nodeContainer[nodeInfo.Name]; !ok {
				nodeContainer[nodeInfo.Name] = []types.CPUMap{}
			}
			nodesInfo[p].Capacity += cap
			volTotal += cap
			nodeContainer[nodeInfo.Name] = append(nodeContainer[nodeInfo.Name], plan...)
		}
	}

	// 裁剪掉不能部署的
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Capacity < nodesInfo[j].Capacity })
	p := sort.Search(len(nodesInfo), func(i int) bool { return nodesInfo[i].Capacity > 0 })
	if p == len(nodesInfo) {
		return nil, nil, 0, types.ErrInsufficientRes
	}

	log.Debugf("[cpuPriorPlan] nodesInfo: %v", nodesInfo)
	return nodesInfo[p:], nodeContainer, volTotal, nil
}

func calcuateCPUPlan(CPUMap types.CPUMap, MemCap int64, cpu float64, memory int64, maxShareCore, coreShare int) (int, []types.CPUMap) {
	host := newHost(CPUMap, coreShare)
	plan := host.getContainerCores(cpu, maxShareCore)
	memLimit := int(MemCap / memory)
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
