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

func (m *magnesium) SelectMemoryNodes(nodesInfo []types.NodeInfo, quota int, memory int64, need int) (map[string]int, error) {
	//TODO not impl yet
	return nil, nil
}

// Select nodes for deploying.
// Use round robin method to select, in order to make scheduler average.
func (m *magnesium) SelectCPUNodes(nodes map[string]types.NodeInfo, quota float64, num int) (map[string][]types.CPUMap, map[string]types.CPUMap, error) {
	m.Lock()
	defer m.Unlock()

	q := int(quota) // 为了和complexscheduler的接口保持一致，quota改为float64
	result := make(map[string][]types.CPUMap)
	changed := make(map[string]types.CPUMap)
	if len(nodes) == 0 {
		return result, nil, fmt.Errorf("No nodes provide to choose some")
	}

	if q > 0 {
		total := totalQuota(nodes)
		if total < num*q {
			return result, nil, fmt.Errorf("Not enough CPUs, total: %d, require: %d", total, num)
		}
	}

done:
	for {
		for nodename, cpumap := range nodes {
			r, err := getQuota(cpumap.CPU, q)
			if err != nil {
				continue
			}
			result[nodename] = append(result[nodename], r)
			if resultLength(result) == num {
				break done
			}
		}
	}

	// 只把有修改的返回回去就可以了, 返回还剩下多少
	for nodename, cpumap := range nodes {
		if _, ok := result[nodename]; ok {
			changed[nodename] = cpumap.CPU
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
func totalQuota(nodes map[string]types.NodeInfo) int {
	value := 0
	for _, cmap := range nodes {
		value += cpuCount(cmap.CPU)
	}
	return value
}

func New() *magnesium {
	return &magnesium{}
}
