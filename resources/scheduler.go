package resources

import (
	"math"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// SelectNodesByResourceRequests select nodes by resource requests
func SelectNodesByResourceRequests(resourceRequests resourcetypes.ResourceRequests, nodeMap map[string]*types.Node) (
	scheduleType types.ResourceType,
	total int,
	planMap map[types.ResourceType]resourcetypes.ResourcePlans,
	err error,
) {
	total = math.MaxInt16
	subTotal := 0
	planMap = map[types.ResourceType]resourcetypes.ResourcePlans{}
	nodesInfo := []types.NodeInfo{}
	for _, node := range nodeMap {
		nodeInfo := types.NodeInfo{
			Name:          node.Name,
			CPUMap:        node.CPU,
			VolumeMap:     node.Volume,
			InitVolumeMap: node.InitVolume,
			MemCap:        node.MemCap,
			StorageCap:    node.StorageCap,
			Capacity:      0,
		}
		nodesInfo = append(nodesInfo, nodeInfo)
	}
	log.Debugf("[SelectNode] nodesInfo: %+v", nodesInfo)
	for _, resourceRequest := range resourceRequests {
		if planMap[resourceRequest.Type()], subTotal, err = resourceRequest.MakeScheduler()(nodesInfo); err != nil {
			return
		}
		total = utils.Min(total, subTotal)

		// calculate schedule type
		if resourceRequest.Type()&types.ResourceCPUBind != 0 {
			scheduleType |= types.ResourceCPU
		}
		if resourceRequest.Type()&types.ResourceScheduledVolume != 0 {
			scheduleType |= types.ResourceVolume
		}
	}

	if scheduleType == 0 {
		scheduleType = types.ResourceMemory
	}
	return
}
