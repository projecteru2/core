/*
CPU 分配的核心算法
*/

package complexscheduler

import (
	"math"
	"sort"

	"gitlab.ricebook.net/platform/core/types"
)

type NodeInfo struct {
	Node string
	NCon int
}

type ByNCon []NodeInfo

func (a ByNCon) Len() int           { return len(a) }
func (a ByNCon) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByNCon) Less(i, j int) bool { return a[i].NCon < a[j].NCon }
func (a ByNCon) volumn() int {
	val := 0
	len := a.Len()
	for i := 0; i < len; i++ {
		val += a[i].NCon
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

func (self *host) GetContainerCores(num float64, maxShareCore int) []types.CPUMap {
	num = num * float64(self.share)

	var full, fragment int
	var result = []types.CPUMap{}
	var fullResult = types.CPUMap{}
	var fragmentResult = []string{}

	full = int(num) / self.share
	fragment = int(num) % self.share

	if full == 0 {
		if maxShareCore == -1 {
			maxShareCore = len(self.full)
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

func AveragePlan(cpu float64, nodes map[string]types.CPUMap, need, maxShareCore, coreShare int) map[string][]types.CPUMap {

	var nodecontainer = map[string][]types.CPUMap{}
	var result = map[string][]types.CPUMap{}
	var host *host
	var plan []types.CPUMap
	var n int
	var nodeinfo ByNCon
	var nodename string

	for node, cpuInfo := range nodes {
		host = newHost(cpuInfo, coreShare)
		plan = host.GetContainerCores(cpu, maxShareCore)
		n = len(plan) // 每个node可以放的容器数
		nodeinfo = append(nodeinfo, NodeInfo{node, n})
		nodecontainer[node] = plan
	}

	if nodeinfo.volumn() < need {
		return nil
	}

	// 排序
	sort.Sort(nodeinfo)

	// 决定分配方案
	allocplan := allocPlan(nodeinfo, need)

	for node, ncon := range allocplan {
		nodename = nodeinfo[node].Node
		result[nodename] = nodecontainer[nodename][:ncon]
	}

	return result
}

func allocPlan(info ByNCon, need int) map[int]int {
	result := make(map[int]int)
	NNode := info.Len()
	var vol int

r:
	for i := 0; i < NNode; i++ {
		vol = info[i].NCon * (NNode - i)
		if vol >= need {
			for j := i; j < NNode; j++ {
				if need <= info[i].NCon {
					result[j] = need
					break r
				}
				result[j] = info[i].NCon
				need -= info[i].NCon
			}
		} else {
			result[i] = info[i].NCon
			need -= info[i].NCon
		}
	}
	return result
}
