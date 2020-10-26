package resources

import "github.com/projecteru2/core/types"

func GetCapacity(nodesInfo []types.NodeInfo) map[string]int {
	capacity := make(map[string]int)
	for _, nodeInfo := range nodesInfo {
		capacity[nodeInfo.Name] = nodeInfo.Capacity
	}
	return capacity
}
