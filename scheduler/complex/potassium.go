package complexscheduler

import (
	"fmt"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

type potassium struct {
	maxshare, sharebase int64
}

// New a potassium
func New(config types.Config) (*potassium, error) {
	return &potassium{config.Scheduler.MaxShare, config.Scheduler.ShareBase}, nil
}

func (m *potassium) MaxIdleNode(nodes []*types.Node) *types.Node {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].CPU.Total() > nodes[j].CPU.Total() })
	return nodes[0]
}

func (m *potassium) SelectMemoryNodes(nodesInfo []types.NodeInfo, rate, memory int64, need int) ([]types.NodeInfo, error) {
	log.Debugf("[SelectMemoryNodes] nodesInfo: %v, rate: %d, memory: %d, need: %d", nodesInfo, rate, memory, need)
	if memory <= 0 {
		return nil, fmt.Errorf("memory must positive")
	}

	nodesInfoLength := len(nodesInfo)

	// 筛选出能满足 CPU 需求的
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].CPURate < nodesInfo[j].CPURate })
	p := sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].CPURate >= rate })
	// p 最大也就是 nodesInfoLength - 1
	if p == nodesInfoLength {
		return nil, fmt.Errorf("Cannot alloc a plan, not enough cpu rate")
	}
	nodesInfoLength -= p
	nodesInfo = nodesInfo[p:]
	log.Debugf("[SelectMemoryNodes] %d nodes has enough cpu rate", nodesInfoLength)

	// 计算是否有足够的内存满足需求
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].MemCap < nodesInfo[j].MemCap })
	p = sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].MemCap >= memory })
	if p == nodesInfoLength {
		return nil, fmt.Errorf("Cannot alloc a plan, not enough memory")
	}
	nodesInfoLength -= p
	nodesInfo = nodesInfo[p:]
	log.Debugf("[SelectMemoryNodes] %d nodes has enough memory", nodesInfoLength)

	volTotal := 0
	capacity := -1
	for i, nodeInfo := range nodesInfo {
		capacity = int(nodeInfo.MemCap / memory)
		if capacity > 0 {
			volTotal += capacity
			nodesInfo[i].Capacity = capacity
		}
	}

	// 继续裁可用节点池子
	log.Debugf("[SelectMemoryNodes] Node info: %v", nodesInfo)
	nodesInfo, err := CommunismDivisionPlan(nodesInfo, need, volTotal)
	if err != nil {
		return nil, err
	}

	// 这里并不需要再次排序了，理论上的排序是基于 Count 得到的 Deploy 最终方案
	log.Debugf("[SelectMemoryNodes] CommunismDivisionPlan: %v", nodesInfo)
	return nodesInfo, nil
}

func (m *potassium) SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, memory int64, need int) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	log.Debugf("[SelectCPUNodes] nodesInfo: %v, cpu: %v, need: %v", nodesInfo, quota, need)
	if quota <= 0 {
		return nil, nil, fmt.Errorf("quota must positive")
	}
	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)

	if len(nodesInfo) == 0 {
		return result, nil, fmt.Errorf("No nodes provide to choose some")
	}

	volTotal, selectedNodesInfo, selectedNodesPool := cpuPriorPlan(quota, memory, nodesInfo, need, m.maxshare, m.sharebase)
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
	log.Debugf("[SelectCPUNodes] result: %v changed %v", result, changed)
	return result, changed, nil
}
