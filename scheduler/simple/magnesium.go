package simplescheduler

import (
	"fmt"
	"sync"

	"gitlab.ricebook.net/platform/core/types"
)

type magnesium struct {
	sync.Mutex
}

// Get a random node.
// Actually it's not random, it's the one with biggest cpu quota.
// Returns the node name.
func (m *magnesium) RandomNode(nodes map[string]types.CPUMap) (string, error) {
	m.Lock()
	defer m.Unlock()

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

func (m *magnesium) SelectMemoryNodes(nodesInfo []types.NodeInfo, quota, memory int64, need int) ([]types.NodeInfo, error) {
	//TODO not impl yet
	return nil, nil
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
