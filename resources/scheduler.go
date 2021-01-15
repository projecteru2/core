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
			return total, plans, errors.WithStack(err)
		}
		plans = append(plans, plan)
		total = utils.Min(total, subTotal)
	}
	return
}
