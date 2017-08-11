package complexscheduler

import (
	"fmt"

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
	var max int64
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

func (m *potassium) SelectMemoryNodes(nodesInfo []types.NodeInfo, rate, memory int64, need int) ([]types.NodeInfo, error) {
	log.Debugf("[SelectMemoryNodes]: nodesInfo: %v, rate: %d, memory: %d, need: %d", nodesInfo, rate, memory, need)

	p := -1
	for i, nodeInfo := range nodesInfo {
		if nodeInfo.CPURate >= rate {
			p = i
			break
		}
	}
	if p == -1 {
		return nil, fmt.Errorf("[SelectMemoryNodes] Cannot alloc a plan, not enough cpu rate")
	}
	log.Debugf("[SelectMemoryNodes] the %d th node has enough cpu rate.", p)

	// 计算是否有足够的内存满足需求
	nodesInfo = nodesInfo[p:]
	volTotal := 0
	p = -1
	for i, nodeInfo := range nodesInfo {
		capacity := int(nodeInfo.MemCap / memory)
		if capacity <= 0 {
			continue
		}
		if p == -1 {
			p = i
		}
		volTotal += capacity
		nodesInfo[i].Capacity = capacity
	}
	if volTotal < need {
		return nil, fmt.Errorf("[SelectMemoryNodes] Cannot alloc a plan, not enough memory, volume %d, need %d", volTotal, need)
	}

	// 继续裁可用节点池子
	nodesInfo = nodesInfo[p:]
	log.Debugf("[SelectMemoryNodes] volumn of each node: %v", nodesInfo)
	nodesInfo, err := CommunismDivisionPlan(nodesInfo, need, volTotal)
	if err != nil {
		return nil, err
	}

	// 这里并不需要再次排序了，理论上的排序是基于 Count 得到的 Deploy 最终方案
	log.Debugf("[SelectMemoryNodes] CommunismDivisionPlan: %v", nodesInfo)
	return nodesInfo, nil
}

//TODO 这里要处理下输入
func (m *potassium) SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, need int) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)

	if len(nodesInfo) == 0 {
		return result, nil, fmt.Errorf("[SelectCPUNodes] No nodes provide to choose some")
	}

	// TODO all core could be shared
	// TODO suppose each core has 10 coreShare
	// TODO change it to be control by parameters
	volTotal, selectedNodesInfo, selectedNodesPool := cpuPriorPlan(quota, nodesInfo, need, -1, 10)
	if volTotal == -1 {
		return nil, nil, fmt.Errorf("Not enough resource")
	}

	selectedNodesInfo, err := CommunismDivisionPlan(selectedNodesInfo, need, volTotal)
	if err != nil {
		return nil, nil, err
	}

	// 只返回有修改的就可以了, 返回有修改的还剩下多少
	for _, selectedNode := range selectedNodesInfo {
		if selectedNode.Deploy <= 0 {
			continue
		}
		cpuList := selectedNodesPool[selectedNode.Name][:selectedNode.Deploy]
		result[selectedNode.Name] = cpuList
		for _, cpu := range cpuList {
			selectedNode.CpuMap.Sub(cpu)
		}
		changed[selectedNode.Name] = selectedNode.CpuMap
	}

	return result, changed, nil
}
