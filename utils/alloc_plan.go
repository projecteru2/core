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

func AllocContainerPlan(nodeInfo ByCoreNum, quota int, memory int64, count int) (map[string]int, error) {
	log.Debugf("[AllocContainerPlan]: nodeInfo: %v, quota: %d, memory: %d, count: %d", nodeInfo, quota, memory, count)

	result := make(map[string]int)
	N := nodeInfo.Len()
	flag := -1

	for i := 0; i < N; i++ {
		if nodeInfo[i].CorePer >= quota {
			flag = i
			break
		}
	}
	if flag == -1 {
		log.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough cpu quota, got flag %d", flag)
		return result, fmt.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough cpu quota")
	}
	log.Debugf("[AllocContainerPlan] the %d node has enough cpu quota.", flag)

	// 计算是否有足够的内存满足需求
	bucket := []NodeInfo{}
	volume := 0
	volNum := []int{} //为了排序
	for i := flag; i < N; i++ {
		temp := int(nodeInfo[i].Memory / memory)
		if temp > 0 {
			volume += temp
			bucket = append(bucket, nodeInfo[i])
			volNum = append(volNum, temp)
		}
	}
	if volume < count {
		log.Errorf("[AllocContainerPlan] Cannot alloc a plan, volume %d, count %d", volume, count)
		return result, fmt.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough memory.")
	}
	log.Debugf("[AlloContainerPlan] volumn of each node: %v", volNum)

	sort.Ints(volNum)
	log.Debugf("[AllocContainerPlan] sorted volumn: %v", volNum)
	plan := allocAlgorithm(volNum, count)

	for i, num := range plan {
		key := bucket[i].Name
		result[key] = num
	}
	log.Debugf("[AllocContainerPlan] allocAlgorithm result: %v", result)
	return result, nil
}

func allocAlgorithm(info []int, need int) map[int]int {
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
			log.Debugf("[AllocContainerPlan] allocAlgorithm outer loop: %d, ave: %d, result: %v", j, ave, result)
		}
		if more == 0 {
			break
		}
		need = -more
	}
	log.Debugf("[AllocContainerPlan] allocAlgorithm: info %v, need %d, made plan: %v", info, need, result)
	return result
}

func GetNodesInfo(cpumemmap map[string]types.CPUAndMem) ByCoreNum {
	result := ByCoreNum{}
	for node, cpuandmem := range cpumemmap {
		result = append(result, NodeInfo{node, len(cpuandmem.CpuMap) * CpuPeriodBase, cpuandmem.MemCap})
	}
	sort.Sort(result)
	return result
}
