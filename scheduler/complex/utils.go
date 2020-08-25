package complexscheduler

import (
	"sort"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func updateNodeInfoCapacity(nodeInfo *types.NodeInfo, capacity int) int {
	if nodeInfo.Capacity == 0 {
		nodeInfo.Capacity = capacity
	} else {
		nodeInfo.Capacity = utils.Min(capacity, nodeInfo.Capacity)
	}
	return nodeInfo.Capacity
}

func onSameSource(plan []types.ResourceMap) bool {
	sourceID := ""
	for _, p := range plan {
		if sourceID == "" {
			sourceID = p.GetResourceID()
		}
		if sourceID != p.GetResourceID() {
			return false
		}
	}
	return true
}

func scoreSort(nodesInfo []types.NodeInfo, byResource types.ResourceType) []types.NodeInfo {
	sort.Slice(nodesInfo, func(i, j int) bool {
		return nodesInfo[i].GetResourceUsage(byResource) < nodesInfo[j].GetResourceUsage(byResource)
	})
	return nodesInfo
}
