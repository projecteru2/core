package utils

import (
	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/core/types"

	"fmt"
	"sort"
)

type NodeInfo struct {
	Name    string
	CorePer int
	Memory  int64
}

type ByCoreNum []NodeInfo

func (a ByCoreNum) Len() int           { return len(a) }
func (a ByCoreNum) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByCoreNum) Less(i, j int) bool { return a[i].CorePer < a[j].CorePer }

type ByMemCap []NodeInfo

func (a ByMemCap) Len() int           { return len(a) }
func (a ByMemCap) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByMemCap) Less(i, j int) bool { return a[i].Memory < a[j].Memory }

func AllocContainerPlan(nodeInfo ByCoreNum, quota int, memory int64, count int) (map[string]int, error) {
	log.Debugf("[AllocContainerPlan]: nodeInfo: %v, quota: %d, memory: %d, count: %d", nodeInfo, quota, memory, count)

	result := make(map[string]int)
	N := nodeInfo.Len()
	firstNodeWithEnoughCPU := -1

	for i := 0; i < N; i++ {
		if nodeInfo[i].CorePer >= quota {
			firstNodeWithEnoughCPU = i
			break
		}
	}
	if firstNodeWithEnoughCPU == -1 {
		log.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough cpu quota, got flag %d", firstNodeWithEnoughCPU)
		return result, fmt.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough cpu quota")
	}
	log.Debugf("[AllocContainerPlan] the %d th node has enough cpu quota.", firstNodeWithEnoughCPU)

	// 计算是否有足够的内存满足需求
	nodeInfoList := ByMemCap{}
	volTotal := 0
	volEachNode := []int{} //为了排序
	for i := firstNodeWithEnoughCPU; i < N; i++ {
		temp := int(nodeInfo[i].Memory / memory)
		if temp > 0 {
			volTotal += temp
			nodeInfoList = append(nodeInfoList, nodeInfo[i])
			volEachNode = append(volEachNode, temp)
		}
	}
	if volTotal < count {
		log.Errorf("[AllocContainerPlan] Cannot alloc a plan, volume %d, count %d", volTotal, count)
		return result, fmt.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough memory.")
	}
	log.Debugf("[AllocContainerPlan] volumn of each node: %v", volEachNode)

	sort.Ints(volEachNode)
	log.Debugf("[AllocContainerPlan] sorted volumn: %v", volEachNode)
	plan, err := allocAlgorithm(volEachNode, count)
	if err != nil {
		log.Errorf("[AllocContainerPlan] %v", err)
		return result, err
	}

	sort.Sort(nodeInfoList)
	log.Debugf("[AllocContainerPlan] sorted nodeInfo: %v, ", nodeInfoList)
	for i, num := range plan {
		key := nodeInfoList[i].Name
		result[key] = num
	}
	log.Debugf("[AllocContainerPlan] allocAlgorithm result: %v", result)
	return result, nil
}

func allocAlgorithm(info []int, need int) (map[int]int, error) {
	// 实际上，这就是精确分配时候的那个分配算法
	// 情景是相同的：我们知道每台机能否分配多少容器
	// 要求我们尽可能平均地分配
	// 算法的正确性我们之前确认了
	// 所以我抄了过来
	result := make(map[int]int)
	nnode := len(info)

	var nodeToUse, more int
	for i := 0; i < nnode; i++ {
		nodeToUse = nnode - i
		ave := need / nodeToUse
		if ave > info[i] {
			ave = 1
		}
		for ; ave < info[i] && ave*nodeToUse < need; ave++ {
		}
		log.Debugf("[AllocContainerPlan] allocAlgorithm outer loop: %d, ave: %d, result: %v", i, ave, result)
		more = ave*nodeToUse - need
		for j := i; nodeToUse != 0; nodeToUse-- {
			if _, ok := result[j]; !ok {
				result[j] = ave
			} else {
				result[j] += ave
			}
			if more > 0 {
				// TODO : 这里应该要有一个随机策略但是我之前的随机策略是有问题的
				//        cmgs 提供了一个思路: 如果当前需要分配的容器数量少于最
				//        小机器的容量的话，我们就直接随机分
				more--
				result[j]--
			} else if more < 0 {
				info[j] -= ave
			}
			j++
			log.Debugf("[AllocContainerPlan] allocAlgorithm inner loop: %d, ave: %d, result: %v", j, ave, result)
		}
		if more == 0 {
			break
		}
		need = -more
	}
	log.Debugf("[AllocContainerPlan] allocAlgorithm info %v, need %d, made plan: %v", info, need, result)
	for _, v := range result {
		if v < 0 {
			// result will not be nil at this situation. So I return nil instead of result
			return nil, fmt.Errorf("allocAlgorithm illegal alloc plan: %v ", result)
		}
	}
	return result, nil
}

func GetNodesInfo(cpumemmap map[string]types.CPUAndMem) ByCoreNum {
	result := ByCoreNum{}
	for node, cpuandmem := range cpumemmap {
		result = append(result, NodeInfo{node, len(cpuandmem.CpuMap) * CpuPeriodBase, cpuandmem.MemCap})
	}
	sort.Sort(result)
	return result
}
