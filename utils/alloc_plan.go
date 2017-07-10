package utils

import (
	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/core/types"

	"fmt"
	"sort"
)

func AllocContainerPlan(nodesInfo []types.NodeInfo, quota int, memory int64, need int) (map[string]int, error) {
	log.Debugf("[AllocContainerPlan]: nodesInfo: %v, quota: %d, memory: %d, need: %d", nodesInfo, quota, memory, need)

	result := map[string]int{}
	var p int = -1
	for i, nodeInfo := range nodesInfo {
		if nodeInfo.CorePer >= quota {
			p = i
			break
		}
	}
	if p == -1 {
		log.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough cpu quota, got flag %d", p)
		return result, fmt.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough cpu quota")
	}
	log.Debugf("[AllocContainerPlan] the %d th node has enough cpu quota.", p)

	// 计算是否有足够的内存满足需求
	nodesInfo = nodesInfo[p:]
	volTotal := 0
	p = -1
	for i, nodeInfo := range nodesInfo {
		capacity := int(nodeInfo.Memory / memory)
		if capacity <= 0 {
			continue
		}
		if p == -1 {
			p = i
		}
		volTotal += capacity
		nodeInfo.Capacity = capacity
	}
	if volTotal < need {
		return result, fmt.Errorf("[AllocContainerPlan] Cannot alloc a plan, not enough memory, volume %d, need %d", volTotal, need)
	}

	// 继续裁可用节点池子
	nodesInfo = nodesInfo[p:]
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Count < nodesInfo[j].Count })
	log.Debugf("[AllocContainerPlan] volumn of each node: %v", nodesInfo)
	nodesInfo, err := AllocByEqualDivision(nodesInfo, need, volTotal)
	if err != nil {
		log.Errorf("[AllocContainerPlan] %v", err)
		return result, err
	}

	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Memory < nodesInfo[j].Memory })
	for _, nodeInfo := range nodesInfo {
		result[nodeInfo.Name] = nodeInfo.Deploy
	}
	log.Debugf("[AllocContainerPlan] allocAlgorithm result: %v", result)
	return result, nil
}

func AllocByEqualDivision(arg []types.NodeInfo, need, volTotal int) ([]types.NodeInfo, error) {
	length := len(arg)
	i := 0
	for need > 0 && volTotal > 0 {
		p := i
		deploy := 0
		differ := 1
		if i < length-1 {
			differ = arg[i+1].Count - arg[i].Count
			i++
		}
		for j := 0; j <= p && need > 0 && differ > 0; j++ {
			// 减枝
			if arg[j].Capacity == 0 {
				continue
			}
			deploy = differ
			if deploy > arg[j].Capacity {
				deploy = arg[j].Capacity
			}
			if deploy > need {
				deploy = need
			}
			arg[j].Deploy += deploy
			arg[j].Capacity -= deploy
			need -= deploy
			volTotal -= deploy
		}
	}
	// 这里 need 一定会为 0 出来，因为 volTotal 在外层保证了一定大于 need
	return arg, nil
}

func GetNodesInfo(cpumemmap map[string]types.CPUAndMem) []types.NodeInfo {
	result := []types.NodeInfo{}
	for node, cpuandmem := range cpumemmap {
		result = append(result, types.NodeInfo{node, len(cpuandmem.CpuMap) * CpuPeriodBase, cpuandmem.MemCap, 0, 0, 0})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Memory < result[j].Memory })
	return result
}
