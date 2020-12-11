package resources

import (
	"math"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// SelectNodesByResourceRequests select nodes by resource requests
func SelectNodesByResourceRequests(resourceRequests resourcetypes.ResourceRequests, nodeMap map[string]*types.Node) (
	scheduleType types.ResourceType,
	total int,
	plans []resourcetypes.ResourcePlans,
	err error,
) {
	total = math.MaxInt64
	scheduleInfos := []resourcetypes.ScheduleInfo{}
	for _, node := range nodeMap {
		scheduleInfo := resourcetypes.ScheduleInfo{
			NodeMeta: node.NodeMeta,
		}
		scheduleInfos = append(scheduleInfos, scheduleInfo)
	}
	log.Debugf("[SelectNodesByResourceRequests] scheduleInfos: %+v", scheduleInfos)
	for _, resourceRequest := range resourceRequests {
		plan, subTotal, err := resourceRequest.MakeScheduler()(scheduleInfos)
		if err != nil {
			return scheduleType, total, plans, errors.WithStack(err)
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
