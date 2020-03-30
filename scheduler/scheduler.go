package scheduler

import "github.com/projecteru2/core/types"

// Scheduler is a scheduler is used to determine which nodes are we gonna use.
// `types.CPUMap` represents the CPU label and remaining quota.
// `nodes` represents node name and the corresponding CPUMap.
type Scheduler interface {
	// select one node from nodes, returns nodename
	// typically used to build image
	MaxIdleNode(nodes []*types.Node) (*types.Node, error)
	SelectStorageNodes(nodesInfo []types.NodeInfo, storage int64) ([]types.NodeInfo, int, error)
	SelectMemoryNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, int, error)
	// select nodes from nodes, return a list of nodenames and the corresponding cpumap, and also the changed nodes with remaining cpumap
	// quota and number must be given, typically used to determine where to deploy
	SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, map[string][]types.CPUMap, int, error)
	// select nodes from nodes, return a list a nodenames and the corresponding volumemap
	SelectVolumeNodes(nodeInfo []types.NodeInfo, vbs types.VolumeBindings) ([]types.NodeInfo, map[string][]types.VolumePlan, int, error)
	// global division
	GlobalDivision(nodesInfo []types.NodeInfo, need, total int, resourceType types.ResourceType) ([]types.NodeInfo, error)
	// common division
	CommonDivision(nodesInfo []types.NodeInfo, need, total int, resourceType types.ResourceType) ([]types.NodeInfo, error)
	// average division
	EachDivision(nodesInfo []types.NodeInfo, need, limit int, resourceType types.ResourceType) ([]types.NodeInfo, error)
	// fill division
	FillDivision(nodesInfo []types.NodeInfo, need, limit int, resourceType types.ResourceType) ([]types.NodeInfo, error)
}
