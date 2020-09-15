package strategy

import (
	"sort"

	"github.com/projecteru2/core/types"
)

const (
	// Auto .
	Auto = "AUTO"
	// Fill .
	Fill = "FILL"
	// Each .
	Each = "EACH"
	// Global .
	Global = "GLOBAL"
)

// Plans .
var Plans = map[string]startegyFunc{
	Auto:   CommunismPlan,
	Fill:   FillPlan,
	Each:   AveragePlan,
	Global: GlobalPlan,
}

type startegyFunc = func(nodesInfo []types.NodeInfo, need, total int, resourceType types.ResourceType) ([]types.NodeInfo, error)

func scoreSort(nodesInfo []types.NodeInfo, byResource types.ResourceType) []types.NodeInfo {
	sort.Slice(nodesInfo, func(i, j int) bool {
		return nodesInfo[i].GetResourceUsage(byResource) < nodesInfo[j].GetResourceUsage(byResource)
	})
	return nodesInfo
}
