package simplescheduler

import (
	"fmt"
	"sort"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/projecteru2/core/types"
)

type magnesium struct {
	sync.Mutex
}

// Get a random node.
// Actually it's not random, it's the one with biggest cpu quota.
// Returns the node name.
func (m *magnesium) MaxIdleNode(nodes []*types.Node) *types.Node {
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].CPU.Total() > nodes[j].CPU.Total() })
	return nodes[0]
}

func (m *magnesium) SelectMemoryNodes(nodesInfo []types.NodeInfo, rate, memory int64, need int) ([]types.NodeInfo, error) {
	log.Debugf("[SelectMemoryNodes]: nodesInfo: %v, rate: %d, memory: %d, need: %d", nodesInfo, rate, memory, need)

	p := -1
	for i, nodeInfo := range nodesInfo {
		if nodeInfo.CPURate >= rate {
			p = i
			break
		}
	}
	if p == -1 {
		return nil, fmt.Errorf("Cannot alloc a plan, not enough cpu rate")
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
		return nil, fmt.Errorf("Cannot alloc a plan, not enough memory, volume %d, need %d", volTotal, need)
	}

	// 一个一个分
	nodesInfo = nodesInfo[p:]
	log.Debugf("[SelectMemoryNodes] volumn of each node: %v", nodesInfo)
	for need > 0 {
		for i := range nodesInfo {
			if need == 0 {
				break
			}
			if nodesInfo[i].Capacity > 0 {
				nodesInfo[i].Capacity--
				nodesInfo[i].Deploy++
				need--
			}
		}
	}

	// 这里并不需要再次排序了，理论上的排序是基于 Count 得到的 Deploy 最终方案
	log.Debugf("[SelectMemoryNodes] CommunismDivisionPlan: %v", nodesInfo)
	return nodesInfo, nil
}

// Select nodes for deploying.
// Use round robin method to select, in order to make scheduler average.
func (m *magnesium) SelectCPUNodes(nodes []types.NodeInfo, quota float64, need int) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	m.Lock()
	defer m.Unlock()

	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)
	if len(nodes) == 0 {
		return result, nil, fmt.Errorf("No nodes provide to choose some")
	}

	if quota > 0 {
		total := totalQuota(nodes)
		requiredCPU := need * int(quota)
		if total < requiredCPU {
			return result, nil, fmt.Errorf("Not enough CPUs, total: %d, require: %d", total, need)
		}
	}

done:
	for {
		for _, node := range nodes {
			nodename := node.Name
			r, err := getQuota(node.CpuMap, int(quota))
			if err != nil {
				continue
			}
			result[nodename] = append(result[nodename], r)
			if resultLength(result) == need {
				break done
			}
		}
	}

	// 只把有修改的返回回去就可以了, 返回还剩下多少
	for _, node := range nodes {
		nodename := node.Name
		if _, ok := result[nodename]; ok {
			changed[nodename] = node.CpuMap
		}
	}
	return result, changed, nil
}

// count result length
func resultLength(result map[string][]types.CPUMap) int {
	length := 0
	for _, list := range result {
		length += len(list)
	}
	return length
}

// count total CPU number
// in fact is the number of all labels
// don't care about the value
func totalQuota(nodes []types.NodeInfo) int {
	value := 0
	for _, cmap := range nodes {
		value += cpuCount(cmap.CpuMap)
	}
	return value
}

func New() *magnesium {
	return &magnesium{}
}
