package resources

import (
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
)

// SelectNodesByResourceRequests select nodes by resource requests
func SelectNodesByResourceRequests(resourceRequests resourcetypes.ResourceRequests, nodeMap map[string]*types.Node) (
	plans []resourcetypes.ResourcePlans,
	err error,
) {
	scheduleInfos := []resourcetypes.ScheduleInfo{}
	for _, node := range nodeMap {
		scheduleInfo := resourcetypes.ScheduleInfo{
			NodeMeta: node.NodeMeta,
		}
		scheduleInfos = append(scheduleInfos, scheduleInfo)
	}
	log.Debugf("[SelectNodesByResourceRequests] scheduleInfos: %+v", scheduleInfos)
	for _, resourceRequest := range resourceRequests {
		plan, _, err := resourceRequest.MakeScheduler()(scheduleInfos)
		if err != nil {
			return plans, err
		}
		plans = append(plans, plan)
	}
	return
}
