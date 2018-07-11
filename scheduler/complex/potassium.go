package complexscheduler

import (
	"fmt"
	"sort"

	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// Potassium is a scheduler
type Potassium struct {
	maxshare, sharebase int64
}

// New a potassium
func New(config types.Config) (*Potassium, error) {
	return &Potassium{config.Scheduler.MaxShare, config.Scheduler.ShareBase}, nil
}

// MaxCPUIdleNode use for build
func (m *Potassium) MaxCPUIdleNode(nodes []*types.Node) *types.Node {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].CPU.Total() > nodes[j].CPU.Total() })
	return nodes[0]
}

// SelectMemoryNodes filter nodes with enough memory
func (m *Potassium) SelectMemoryNodes(nodesInfo []types.NodeInfo, rate, memory int64) ([]types.NodeInfo, int, error) {
	log.Debugf("[SelectMemoryNodes] nodesInfo: %v, rate: %d, memory: %d", nodesInfo, rate, memory)
	if memory <= 0 {
		return nil, 0, fmt.Errorf("memory must positive")
	}

	nodesInfoLength := len(nodesInfo)

	// 筛选出能满足 CPU 需求的
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].CPURate < nodesInfo[j].CPURate })
	p := sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].CPURate >= rate })
	// p 最大也就是 nodesInfoLength - 1
	if p == nodesInfoLength {
		return nil, 0, fmt.Errorf("Cannot alloc a plan, not enough cpu rate")
	}
	nodesInfoLength -= p
	nodesInfo = nodesInfo[p:]
	log.Debugf("[SelectMemoryNodes] %d nodes has enough cpu rate", nodesInfoLength)

	// 计算是否有足够的内存满足需求
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].MemCap < nodesInfo[j].MemCap })
	p = sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].MemCap >= memory })
	if p == nodesInfoLength {
		return nil, 0, fmt.Errorf("Cannot alloc a plan, not enough memory")
	}
	nodesInfoLength -= p
	nodesInfo = nodesInfo[p:]
	log.Debugf("[SelectMemoryNodes] %d nodes has enough memory", nodesInfoLength)

	// 这里 memCap 一定是大于 memory 的所以不用判断 cap 内容
	volTotal := 0
	for i, nodeInfo := range nodesInfo {
		capacity := int(nodeInfo.MemCap / memory)
		volTotal += capacity
		nodesInfo[i].Capacity = capacity
	}
	log.Debugf("[SelectMemoryNodes] Node info: %v", nodesInfo)
	return nodesInfo, volTotal, nil
}

// SelectCPUNodes select nodes with enough cpus
func (m *Potassium) SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, map[string][]types.CPUMap, int, error) {
	log.Debugf("[SelectCPUNodes] nodesInfo: %v, cpu: %v", nodesInfo, quota)
	if quota <= 0 {
		return nil, nil, 0, fmt.Errorf("quota must positive")
	}
	if len(nodesInfo) == 0 {
		return nil, nil, 0, fmt.Errorf("No nodes provide to choose some")
	}
	return cpuPriorPlan(quota, memory, nodesInfo, m.maxshare, m.sharebase)
}

// CommonDivision deploy containers by their deploy status
// 部署完 N 个后全局尽可能平均
func (m *Potassium) CommonDivision(nodesInfo []types.NodeInfo, need, total int) ([]types.NodeInfo, error) {
	if total < need {
		return nil, fmt.Errorf("Not enough resource need: %d, vol: %d", need, total)
	}
	return CommunismDivisionPlan(nodesInfo, need)
}

// EachDivision deploy containers by each node
// 容量够的机器每一台部署 N 个
func (m *Potassium) EachDivision(nodesInfo []types.NodeInfo, need, total int) ([]types.NodeInfo, error) {
	if total < need {
		return nil, fmt.Errorf("Not enough resource need: %d, vol: %d", need, total)
	}
	return AveragePlan(nodesInfo, need)
}

// FillDivision deploy containers fill nodes by count
// 根据之前部署的策略每一台补充到 N 个，超过 N 个忽略
func (m *Potassium) FillDivision(nodesInfo []types.NodeInfo, need, total int) ([]types.NodeInfo, error) {
	if total < need {
		return nil, fmt.Errorf("Not enough resource need: %d, vol: %d", need, total)
	}
	return FillPlan(nodesInfo, need)
}
