package types

import (
	"context"

	"github.com/projecteru2/core/types"
)

const supported = 3

// ResourceRequests .
type ResourceRequests [supported]ResourceRequest

// MainResourceType calculates main resource type
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

// MainRateOnNode calculates resource consumption rate for this request on a node
func (rr ResourceRequests) MainRateOnNode(node types.Node) (rate float64) {
	mainType := rr.MainResourceType()
	for _, req := range rr {
		if req.Type()&mainType != 0 {
			rate += req.Rate(node)
		}
	}
	return
}

// MainUsageOnNode calculates current resource usage rate based on MainType
func (rr ResourceRequests) MainUsageOnNode(node types.Node) (usage float64) {
	mainType := rr.MainResourceType()
	for t, use := range node.ResourceUsages() {
		if t&mainType != 0 {
			usage += use
		}
	}
	return
}

// ResourceRequest .
type ResourceRequest interface {
	Type() types.ResourceType
	Validate() error
	MakeScheduler() SchedulerV2
	Rate(types.Node) float64
}

// SchedulerV2 .
type SchedulerV2 func(context.Context, []ScheduleInfo) (ResourcePlans, int, error)

// DispenseOptions .
type DispenseOptions struct {
	*types.Node
	Index int
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
