package utils

import "github.com/projecteru2/core/types"

// GetCapacity .
func GetCapacity(nodesInfo []types.NodeInfo) map[string]int {
	capacity := make(map[string]int)
	for _, nodeInfo := range nodesInfo {
		capacity[nodeInfo.Name] = nodeInfo.Capacity
	}
	return capacity
}
