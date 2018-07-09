package scheduler

import "github.com/projecteru2/core/types"

const (
	//CPU_PRIOR define cpu select
	CPU_PRIOR = "CPU"
	//MEMORY_PRIOR define mem select
	MEMORY_PRIOR = "MEM"
)

// Scheduler is a scheduler is used to determine which nodes are we gonna use.
// `types.CPUMap` represents the CPU label and remaining quota.
// `nodes` represents node name and the corresponding CPUMap.
type Scheduler interface {
	// select one node from nodes, returns nodename
	// typically used to build image
	MaxCPUIdleNode(nodes []*types.Node) *types.Node
	SelectMemoryNodes(nodesInfo []types.NodeInfo, quota, memory int64) ([]types.NodeInfo, int, error)
	// select nodes from nodes, return a list of nodenames and the corresponding cpumap, and also the changed nodes with remaining cpumap
	// quota and number must be given, typically used to determine where to deploy
	SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, map[string][]types.CPUMap, int, error)
	// make CPU Plan
	MakeCPUPlan(nodesInfo []types.NodeInfo, nodePlans map[string][]types.CPUMap) (map[string][]types.CPUMap, map[string]types.CPUMap)
	// common division
	CommonDivision(nodesInfo []types.NodeInfo, need, total int) ([]types.NodeInfo, error)
	// average division
	EachDivision(nodesInfo []types.NodeInfo, need, total int) ([]types.NodeInfo, error)
}
