package types

import (
	"github.com/projecteru2/core/types"
)

const supported = 3

// ResourceRequests .
type ResourceRequests [supported]ResourceRequest

func (rr ResourceRequests) MainResourceType() (mainType types.ResourceType) {
	for _, req := range rr {
		if req.Type()&types.ResourceCPUBind != 0 {
			mainType |= types.ResourceCPU
		}
		if req.Type()&types.ResourceScheduledVolume != 0 {
			mainType |= types.ResourceVolume
		}
	}
	if mainType == 0 {
		mainType = types.ResourceMemory
	}
	return mainType
}

func (rr ResourceRequests) MainRateOnNode(node types.Node) (rate float64) {
	mainType := rr.MainResourceType()
	for _, req := range rr {
		if req.Type()&mainType != 0 {
			rate += req.Rate(node)
		}
	}
	return
}

func (rr ResourceRequests) MainUsageOnNode(node types.Node) (usage float64) {
	mainType := rr.MainResourceType()
	for t, use := range node.ResourceUsages() {
		if t&mainType != 0 {
			usage += use
		}
	}
	return
}

// ResourceRequirement .
type ResourceRequest interface {
	Type() types.ResourceType
	Validate() error
	MakeScheduler() SchedulerV2
	Rate(types.Node) float64
}

// SchedulerV2 .
type SchedulerV2 func([]ScheduleInfo) (ResourcePlans, int, error)

// DispenseOptions .
type DispenseOptions struct {
	*types.Node
	Index            int
	ExistingInstance *types.Workload
}

// ResourcePlans .
type ResourcePlans interface {
	Type() types.ResourceType
	Capacity() map[string]int
	ApplyChangesOnNode(*types.Node, ...int)
	RollbackChangesOnNode(*types.Node, ...int)
	Dispense(DispenseOptions, *types.ResourceMeta) (*types.ResourceMeta, error)
}

// ScheduleInfo for scheduler
type ScheduleInfo struct {
	types.NodeMeta

	CPUPlan     []types.CPUMap
	VolumePlans []types.VolumePlan // {{"AUTO:/data:rw:1024": "/mnt0:/data:rw:1024"}}
	Capacity    int                // 可以部署几个
}
