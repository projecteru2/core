package scheduler

import "gitlab.ricebook.net/platform/core/types"

// A scheduler is used to determine which nodes are we gonna use.
// `types.CPUMap` represents the CPU label and remaining quota.
// `nodes` represents node name and the corresponding CPUMap.
type Scheduler interface {
	// select one node from nodes, returns nodename
	// typically used to build image
	RandomNode(nodes map[string]types.CPUMap) (string, error)
	SelectMemoryNodes(nodesInfo []types.NodeInfo, quota int, memory int64, need int) (map[string]int, error)
	// select nodes from nodes, return a list of nodenames and the corresponding cpumap, and also the changed nodes with remaining cpumap
	// quota and number must be given, typically used to determine where to deploy
	SelectCPUNodes(nodes map[string]types.CPUMap, quota float64, need int) (map[string][]types.CPUMap, map[string]types.CPUMap, error)
}
