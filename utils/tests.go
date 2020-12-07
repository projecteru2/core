package utils

import (
	"fmt"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
)

// GenerateScheduleInfos generate nodes
func GenerateScheduleInfos(nums, cores int, memory, storage int64, shares int) []resourcetypes.ScheduleInfo {
	var name string
	scheduleInfos := []resourcetypes.ScheduleInfo{}

	for i := 0; i < nums; i++ {
		name = fmt.Sprintf("n%d", i)

		cpumap := types.CPUMap{}
		for j := 0; j < cores; j++ {
			coreName := fmt.Sprintf("%d", j)
			cpumap[coreName] = int64(shares)
		}
		scheduleInfo := resourcetypes.ScheduleInfo{
			NodeMeta: types.NodeMeta{
				CPU:        cpumap,
				MemCap:     memory,
				StorageCap: storage,
				Name:       name,
			},
		}
		scheduleInfos = append(scheduleInfos, scheduleInfo)
	}
	return scheduleInfos
}
