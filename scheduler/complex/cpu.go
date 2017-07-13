/*
CPU 分配的核心算法
*/

package complexscheduler

import (
	"math"
	"sort"

	"gitlab.ricebook.net/platform/core/types"
)

func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func abs(a int64) int64 {
	if a < 0 {
		return -a
	}
	return a
}

type host struct {
	full     types.CPUMap
	fragment types.CPUMap
	share    int64
}

func newHost(cpuInfo types.CPUMap, share int64) *host {
	result := &host{
		share:    share,
		full:     types.CPUMap{},
		fragment: types.CPUMap{},
	}
	for no, pieces := range cpuInfo {
		if pieces == share {
			result.full[no] = pieces
		} else {
			result.fragment[no] = pieces
		}
	}

	return result
}

func (self *host) calcuatePiecesCores(full, fragment, maxShareCore int64) {
	var fullResultNum, fragmentResultNum, canDeployNum, baseLine int64
	var fragmentBaseResult, lenFull, count, num, flag, b, baseContainers int64

	count = int64(len(self.fragment))
	lenFull = int64(len(self.full))
	if maxShareCore == -1 {
		maxShareCore = lenFull - count - full // 减枝，M == N 的情况下预留至少一个 full 量的核数
	} else {
		maxShareCore -= count
	}

	fullResultNum = lenFull / full
	fragmentBaseResult = 0
	for _, pieces := range self.fragment {
		fragmentBaseResult += pieces / fragment
	}
	baseLine = min(fullResultNum, fragmentBaseResult)

	num = 0
	flag = math.MaxInt64
	baseContainers = self.share / fragment
	var i int64
	for i = 1; i < maxShareCore+1; i++ {
		fullResultNum = (lenFull - i) / full
		fragmentResultNum = fragmentBaseResult + i*baseContainers
		// 剪枝，2者结果相近的时候最优
		b = abs(fullResultNum - fragmentResultNum)
		if b > flag {
			break
		}
		flag = b
		// 计算可以部署的量
		canDeployNum = min(fullResultNum, fragmentResultNum)
		if canDeployNum > baseLine {
			num = i
			baseLine = canDeployNum
		}
	}

	num += count
	for no, pieces := range self.full {
		if count == num {
			break
		}
		self.fragment[no] = pieces
		count += 1
		delete(self.full, no)
	}
}

func (self *host) getContainerCores(num float64, maxShareCore int64) []types.CPUMap {
	num = num * float64(self.share)

	var full, fragment, i int64
	var result = []types.CPUMap{}
	var fullResult = types.CPUMap{}
	var fragmentResult = []string{}

	full = int64(num) / self.share
	fragment = int64(num) % self.share

	if full == 0 {
		if maxShareCore == -1 {
			// 这个时候就把所有的核都当成碎片核
			maxShareCore = int64(len(self.full)) + int64(len(self.fragment))
		}
		for no, pieces := range self.full {
			if int64(len(self.fragment)) >= maxShareCore {
				break
			}
			self.fragment[no] = pieces
			delete(self.full, no)
		}
		fragmentResult = self.getFragmentResult(fragment)
		for _, no := range fragmentResult {
			result = append(result, types.CPUMap{no: fragment})
		}
		return result
	}

	if fragment == 0 {
		n := int64(len(self.full)) / full
		for i = 0; i < n; i++ {
			fullResult = self.getFullResult(full)
			result = append(result, fullResult)
		}
		return result
	}

	// 算出最优的碎片核和整数核组合
	self.calcuatePiecesCores(full, fragment, maxShareCore)
	fragmentResult = self.getFragmentResult(fragment)
	for _, no := range fragmentResult {
		fullResult = self.getFullResult(full)
		if int64(len(fullResult)) != full { // 可能整数核不够用了结果并不一定可靠必须再判断一次
			return result // 减枝这时候整数核一定不够用了，直接退出，这样碎片核和整数核的计算就完成了
		}
		fullResult[no] = fragment
		result = append(result, fullResult)
	}
	return result
}

func (self *host) getFragmentResult(fragment int64) []string {
	var result = []string{}
	var i int64
	for no, pieces := range self.fragment {
		for i = 0; i < pieces/fragment; i++ {
			result = append(result, no)
		}
	}
	return result
}

func (self *host) getFullResult(full int64) types.CPUMap {
	var result = types.CPUMap{}
	for no, pieces := range self.full {
		result[no] = pieces   // 分配一整个核
		delete(self.full, no) // 干掉这个可用资源
		if int64(len(result)) == full {
			break
		}
	}
	return result
}

func cpuPriorPlan(cpu float64, nodesInfo []types.NodeInfo, need int, maxShareCore, coreShare int64) (
	int, []types.NodeInfo, map[string][]types.CPUMap) {

	var nodeContainer = map[string][]types.CPUMap{}
	var host *host
	var plan []types.CPUMap

	// TODO weird check cpu < 0.01

	volTotal := 0
	for p, nodeInfo := range nodesInfo {
		host = newHost(nodeInfo.CpuMap, coreShare)
		plan = host.getContainerCores(cpu, maxShareCore)
		cap := len(plan) // 每个node可以放的容器数
		if cap > 0 {
			nodesInfo[p].Capacity = cap
			nodeContainer[nodeInfo.Name] = plan
			volTotal += cap
		}
	}

	if volTotal < need {
		return -1, nil, nil
	}

	// 裁剪掉不能部署的
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Capacity < nodesInfo[j].Capacity })
	var p int
	for p = 0; p < len(nodesInfo); p++ {
		if nodesInfo[p].Capacity > 0 {
			break
		}
	}

	return volTotal, nodesInfo[p:], nodeContainer
}
