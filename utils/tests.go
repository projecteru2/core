package utils

import (
	"fmt"

	"github.com/projecteru2/core/types"
)

// GenerateNodes generate nodes
func GenerateNodes(nums, cores int, memory, storage int64, shares int) []types.NodeInfo {
	var name string
	nodes := []types.NodeInfo{}

	for i := 0; i < nums; i++ {
		name = fmt.Sprintf("n%d", i)

		cpumap := types.CPUMap{}
		for j := 0; j < cores; j++ {
			coreName := fmt.Sprintf("%d", j)
			cpumap[coreName] = int64(shares)
		}
		nodeInfo := types.NodeInfo{
			NodeMeta: types.NodeMeta{
				CPU:        cpumap,
				MemCap:     memory,
				StorageCap: storage,
				Name:       name,
			},
		}
		nodes = append(nodes, nodeInfo)
	}
	return nodes
}
