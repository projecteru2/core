package resources

import (
	"context"

	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
)

// SelectNodesByResourceRequests select nodes by resource requests
func SelectNodesByResourceRequests(ctx context.Context, resourceRequests resourcetypes.ResourceRequests, nodeMap map[string]*types.Node) (
	plans []resourcetypes.ResourcePlans,
	err error,
) {
	scheduleInfos := []resourcetypes.ScheduleInfo{}
	for _, node := range nodeMap {
		nodeMeta, err := node.NodeMeta.DeepCopy()
		if err != nil {
			return nil, err
		}
		scheduleInfo := resourcetypes.ScheduleInfo{
			NodeMeta: nodeMeta,
		}
		scheduleInfos = append(scheduleInfos, scheduleInfo)
	}
	log.Debugf(ctx, "[SelectNodesByResourceRequests] scheduleInfos: %+v", scheduleInfos)
	for _, resourceRequest := range resourceRequests {
		plan, _, err := resourceRequest.MakeScheduler()(ctx, scheduleInfos)
		if err != nil {
			return plans, err
		}
		plans = append(plans, plan)
	}
	return
}
