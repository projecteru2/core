package complexscheduler

import (
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

func mergePlans(plans1 [][]types.ResourceMap, plans2 [][]types.ResourceMap, cap int) (dist [][]types.ResourceMap) {
	if plans1 == nil && plans2 == nil {
		return nil
	}
	if plans1 != nil {
		dist = plans1[:cap]
	}
	if plans2 != nil {
		for i := 0; i < cap; i++ {
			dist[i] = append(dist[i], plans2[i]...)
		}
	}
	return
}
