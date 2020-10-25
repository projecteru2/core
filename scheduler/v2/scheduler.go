package v2

import (
	"math"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// SchedulerV2 .
type SchedulerV2 func([]types.NodeInfo) (ResourcePlans, int, error)

// SelectNodes .
func SelectNodes(resourceApps ResourceApplications, nodeMap map[string]*types.Node) (planMap map[types.ResourceType]ResourcePlans, total int, scheduleTypes types.ResourceType, err error) {
	total = math.MaxInt16
	subTotal := 0
	planMap = make(map[types.ResourceType]ResourcePlans)
	nodesInfo := getNodesInfo(nodeMap)
	log.Debugf("[SelectNode] nodesInfo: %+v", nodesInfo)
	for _, app := range resourceApps {
		scheduler := app.MakeScheduler()
		if planMap[app.Type()], subTotal, err = scheduler(nodesInfo); err != nil {
			return
		}
		total = utils.Min(total, subTotal)

		// calculate schedule type
		if app.Type()&types.ResourceCPUBind != 0 {
			scheduleTypes |= types.ResourceCPU
		}
		if app.Type()&types.ResourceScheduledVolume != 0 {
			scheduleTypes |= types.ResourceVolume
		}
	}

	if scheduleTypes == 0 {
		scheduleTypes = types.ResourceMemory
	}
	return
}

func getNodesInfo(nodes map[string]*types.Node) []types.NodeInfo {
	result := []types.NodeInfo{}
	for _, node := range nodes {
		nodeInfo := types.NodeInfo{
			Name:          node.Name,
			CPUMap:        node.CPU,
			VolumeMap:     node.Volume,
			InitVolumeMap: node.InitVolume,
			MemCap:        node.MemCap,
			StorageCap:    node.StorageCap,
			Capacity:      0,
		}
		result = append(result, nodeInfo)
	}
	return result
}
