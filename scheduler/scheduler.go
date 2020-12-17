package scheduler

import (
	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
)

var (
	scheduler Scheduler
)

// Scheduler is a scheduler is used to determine which nodes are we gonna use.
// `types.CPUMap` represents the CPU label and remaining quota.
// `nodes` represents node name and the corresponding CPUMap.
type Scheduler interface {
	// select one node from nodes, returns nodename
	// typically used to build image
	MaxIdleNode(nodes []*types.Node) (*types.Node, error)
	SelectStorageNodes(scheduleInfos []resourcetypes.ScheduleInfo, storage int64) ([]resourcetypes.ScheduleInfo, int, error)
	SelectMemoryNodes(scheduleInfos []resourcetypes.ScheduleInfo, quota float64, memory int64) ([]resourcetypes.ScheduleInfo, int, error)
	// select nodes from nodes, return a list of nodenames and the corresponding cpumap, and also the changed nodes with remaining cpumap
	// quota and number must be given, typically used to determine where to deploy
	SelectCPUNodes(scheduleInfos []resourcetypes.ScheduleInfo, quota float64, memory int64) ([]resourcetypes.ScheduleInfo, map[string][]types.CPUMap, int, error)
	// ReselectCPUNodes is used for realloc only
	ReselectCPUNodes(scheduleInfo resourcetypes.ScheduleInfo, CPU types.CPUMap, quota float64, memory int64) (resourcetypes.ScheduleInfo, map[string][]types.CPUMap, int, error)
	// select nodes from nodes, return a list a nodenames and the corresponding volumemap
	SelectVolumeNodes(scheduleInfo []resourcetypes.ScheduleInfo, vbs types.VolumeBindings) ([]resourcetypes.ScheduleInfo, map[string][]types.VolumePlan, int, error)
}

// InitSchedulerV1 .
func InitSchedulerV1(s Scheduler) {
	scheduler = s
}

// GetSchedulerV1 .
func GetSchedulerV1() (Scheduler, error) {
	if scheduler == nil {
		return nil, errors.WithStack(errors.Errorf("potassium not initiated"))
	}
	return scheduler, nil
}
