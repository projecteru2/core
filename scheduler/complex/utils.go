package complexscheduler

import (
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func updateScheduleInfoCapacity(scheduleInfo *resourcetypes.ScheduleInfo, capacity int) int {
	if scheduleInfo.Capacity == 0 {
		scheduleInfo.Capacity = capacity
	} else {
		scheduleInfo.Capacity = utils.Min(capacity, scheduleInfo.Capacity)
	}
	return scheduleInfo.Capacity
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
