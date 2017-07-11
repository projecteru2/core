/*
CPU 分配的核心算法
*/

package complexscheduler

import (
	"math"
	"sort"

	"gitlab.ricebook.net/platform/core/types"
)

func volumn(nodeInfo []types.NodeInfo) int {
	val := 0
	for _, v := range nodeInfo {
		val += v.Capacity
	}
	return val
}

type host struct {
	full     types.CPUMap
	fragment types.CPUMap
	share    int
}

func newHost(cpuInfo types.CPUMap, share int) *host {
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

func (self *host) calcuatePiecesCores(full int, fragment int, maxShareCore int) {
	var fullResultNum, fragmentResultNum, canDeployNum, baseLine int
	var fragmentBaseResult, count, num, flag, b, baseContainers int

	count = len(self.fragment)
	if maxShareCore == -1 {
		maxShareCore = len(self.full) - count - full // 减枝，M == N 的情况下预留至少一个 full 量的核数
	} else {
		maxShareCore -= count
	}

	fullResultNum = len(self.full) / full
	fragmentBaseResult = 0
	for _, pieces := range self.fragment {
		fragmentBaseResult += pieces / fragment
	}
	baseLine = min(fullResultNum, fragmentBaseResult)

	num = 0
	flag = math.MaxInt64
	baseContainers = self.share / fragment
	for i := 1; i < maxShareCore+1; i++ {
		fullResultNum = (len(self.full) - i) / full
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

func (self *host) getContainerCores(num float64, maxShareCore int) []types.CPUMap {
	num = num * float64(self.share)

	var full, fragment int
	var result = []types.CPUMap{}
	var fullResult = types.CPUMap{}
	var fragmentResult = []string{}

	full = int(num) / self.share
	fragment = int(num) % self.share

	if full == 0 {
		if maxShareCore == -1 {
			// 这个时候就把所有的核都当成碎片核
			maxShareCore = len(self.full) + len(self.fragment)
		}
		for no, pieces := range self.full {
			if len(self.fragment) >= maxShareCore {
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
		n := len(self.full) / full
		for i := 0; i < n; i++ {
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
		if len(fullResult) != full { // 可能整数核不够用了结果并不一定可靠必须再判断一次
			return result // 减枝这时候整数核一定不够用了，直接退出，这样碎片核和整数核的计算就完成了
		}
		fullResult[no] = fragment
		result = append(result, fullResult)
	}
	return result
}

func (self *host) getFragmentResult(fragment int) []string {
	var result = []string{}
	for no, pieces := range self.fragment {
		for i := 0; i < pieces/fragment; i++ {
			result = append(result, no)
		}
	}
	return result
}

func (self *host) getFullResult(full int) types.CPUMap {
	var result = types.CPUMap{}
	for no, pieces := range self.full {
		result[no] = pieces   // 分配一整个核
		delete(self.full, no) // 干掉这个可用资源
		if len(result) == full {
			break
		}
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func averagePlan(cpu float64, nodes map[string]types.CPUMap, need, maxShareCore, coreShare int) map[string][]types.CPUMap {

	var nodecontainer = map[string][]types.CPUMap{}
	var result = map[string][]types.CPUMap{}
	var host *host
	var plan []types.CPUMap
	var n int
	var nodeInfo []types.NodeInfo
	var nodeName string

	if cpu < 0.01 {
		resultLength := 0
	r:
		for {
			for nodeName, _ := range nodes {
				result[nodeName] = append(result[nodeName], nil)
				resultLength++
			}
			if resultLength == need {
				break r
			}
		}
		return result
	}

	for node, cpuInfo := range nodes {
		host = newHost(cpuInfo, coreShare)
		plan = host.getContainerCores(cpu, maxShareCore)
		n = len(plan) // 每个node可以放的容器数
		if n > 0 {
			nodeInfo = append(nodeInfo, types.NodeInfo{Name: node, Capacity: n})
			nodecontainer[node] = plan
		}
	}

	if volumn(nodeInfo) < need {
		return nil
	}

	// 排序
	sort.Slice(nodeInfo, func(i, j int) bool {
		return nodeInfo[i].Capacity < nodeInfo[j].Capacity
	})

	// 决定分配方案
	allocplan := allocPlan(nodeInfo, need)
	for node, ncon := range allocplan {
		if ncon > 0 {
			nodeName = nodeInfo[node].Name
			result[nodeName] = nodecontainer[nodeName][:ncon]
		}
	}

	return result
}

func allocPlan(info []types.NodeInfo, need int) map[int]int {
	result := make(map[int]int)
	NNode := len(info)

	var nodeToUse, more int
	for i := 0; i < NNode; i++ {
		nodeToUse = NNode - i
		ave := need / nodeToUse
		if ave > info[i].Capacity {
			ave = 1
		}
		for ; ave < info[i].Capacity && ave*nodeToUse < need; ave++ {
		}
		more = ave*nodeToUse - need
		for j := i; nodeToUse != 0; nodeToUse-- {
			if _, ok := result[j]; !ok {
				result[j] = ave
			} else {
				result[j] += ave
			}
			if more > 0 {
				more--
				result[j]--
			} else if more < 0 {
				info[j].Capacity -= ave
			}
			j++
		}
		if more == 0 {
			break
		}
		need = -more
	}
	return result
}
