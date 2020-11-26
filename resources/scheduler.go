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
	plans []resourcetypes.ResourcePlans,
	err error,
) {
	total = math.MaxInt64
	nodesInfo := []types.NodeInfo{}
	for _, node := range nodeMap {
		nodeInfo := types.NodeInfo{
			NodeMeta: node.NodeMeta,
		}
		nodesInfo = append(nodesInfo, nodeInfo)
	}
	log.Debugf("[SelectNodesByResourceRequests] nodesInfo: %+v", nodesInfo)
	for _, resourceRequest := range resourceRequests {
		plan, subTotal, err := resourceRequest.MakeScheduler()(nodesInfo)
		if err != nil {
			return scheduleType, total, plans, err
		}
		plans = append(plans, plan)
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
	return // nolint:nakedret
}
