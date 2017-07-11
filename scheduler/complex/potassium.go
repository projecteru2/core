package complexscheduler

import (
	"fmt"
	"sort"

	log "github.com/Sirupsen/logrus"
	"gitlab.ricebook.net/platform/core/types"
)

type potassium struct {
}

func New(config types.Config) (*potassium, error) {
	return &potassium{}, nil
}

func (m *potassium) RandomNode(nodes map[string]types.CPUMap) (string, error) {
	var nodename string
	if len(nodes) == 0 {
		return nodename, fmt.Errorf("No nodes provide to choose one")
	}
	max := 0
	for name, cpumap := range nodes {
		total := cpumap.Total()
		if total > max {
			max = total
			nodename = name
		}
	}

	// doesn't matter if max is still 0
	// which means no resource available
	return nodename, nil
}

func (m *potassium) SelectMemoryNodes(nodesInfo []types.NodeInfo, quota int, memory int64, need int) (map[string]int, error) {
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
	nodesInfo, err := equalDivisionPlan(nodesInfo, need, volTotal)
	if err != nil {
		return result, err
	}

	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Memory < nodesInfo[j].Memory })
	for _, nodeInfo := range nodesInfo {
		result[nodeInfo.Name] = nodeInfo.Deploy
	}
	log.Debugf("[AllocContainerPlan] allocAlgorithm result: %v", result)
	return result, nil
}

func (m *potassium) SelectCPUNodes(nodes map[string]types.CPUMap, quota float64, need int) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)

	if len(nodes) == 0 {
		return result, nil, fmt.Errorf("No nodes provide to choose some")
	}

	// all core could be shared
	// suppose each core has 10 coreShare
	// TODO: change it to be control by parameters
	result = averagePlan(quota, nodes, need, -1, 10)
	if result == nil {
		return nil, nil, fmt.Errorf("Not enough resource")
	}

	// 只返回有修改的就可以了, 返回有修改的还剩下多少
	for nodename, cpuList := range result {
		node, ok := nodes[nodename]
		if !ok {
			continue
		}
		for _, cpu := range cpuList {
			node.Sub(cpu)
		}
		changed[nodename] = node
	}
	return result, changed, nil
}
