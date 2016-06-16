package simplescheduler

import (
	"fmt"
	"sync"

	"gitlab.ricebook.net/platform/core/types"
)

type Magnesium struct {
	sync.Mutex
}

// Get a random node.
// Actually it's not random, it's the one with biggest cpu quota.
// Returns the node name.
func (m *Magnesium) RandomNode(nodes map[string]types.CPUMap) (string, error) {
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

// Select nodes for deploying.
// Use round robin method to select, in order to make scheduler average.
func (m *Magnesium) SelectNodes(nodes map[string]types.CPUMap, quota int, num int) (map[string][]types.CPUMap, error) {
	m.Lock()
	defer m.Unlock()

	result := make(map[string][]types.CPUMap)
	if len(nodes) == 0 {
		return result, fmt.Errorf("No nodes provide to choose some")
	}

	total := totalQuota(nodes)
	if total < num {
		return result, fmt.Errorf("Not enough CPUs, total: %d, require: %d", total, num)
	}

done:
	for {
		for nodename, cpumap := range nodes {
			r, err := getQuota(cpumap, quota)
			if err != nil {
				continue
			}
			result[nodename] = append(result[nodename], r)
			if resultLength(result) == num {
				break done
			}
		}
	}
	return result, nil
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
func totalQuota(nodes map[string]types.CPUMap) int {
	value := 0
	for _, cmap := range nodes {
		value += cpuCount(cmap)
	}
	return value
}
