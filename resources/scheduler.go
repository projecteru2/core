package resources

import (
	"math"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// SelectNodes .
func SelectNodes(rrs resourcetypes.ResourceRequirements, nodeMap map[string]*types.Node) (planMap map[types.ResourceType]resourcetypes.ResourcePlans, total int, scheduleTypes types.ResourceType, err error) {
	total = math.MaxInt16
	subTotal := 0
	planMap = make(map[types.ResourceType]resourcetypes.ResourcePlans)
	nodesInfo := getNodesInfo(nodeMap)
	log.Debugf("[SelectNode] nodesInfo: %+v", nodesInfo)
	for _, rr := range rrs {
		scheduler := rr.MakeScheduler()
		if planMap[rr.Type()], subTotal, err = scheduler(nodesInfo); err != nil {
			return
		}
		total = utils.Min(total, subTotal)

		// calculate schedule type
		if rr.Type()&types.ResourceCPUBind != 0 {
			scheduleTypes |= types.ResourceCPU
		}
		if rr.Type()&types.ResourceScheduledVolume != 0 {
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
