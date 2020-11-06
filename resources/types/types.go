package types

import (
	"github.com/projecteru2/core/types"
)

const supported = 3

// ResourceRequests .
type ResourceRequests [supported]ResourceRequest

// ResourceRequirement .
type ResourceRequest interface {
	Type() types.ResourceType
	Validate() error
	MakeScheduler() SchedulerV2
	Rate(types.Node) float64
}

// SchedulerV2 .
type SchedulerV2 func([]types.NodeInfo) (ResourcePlans, int, error)

// DispenseOptions .
type DispenseOptions struct {
	*types.Node
	ExistingInstances  []*types.Container
	Index              int
	HardVolumeBindings types.VolumeBindings
}

// ResourcePlans .
type ResourcePlans interface {
	Type() types.ResourceType
	Capacity() map[string]int
	ApplyChangesOnNode(*types.Node, ...int)
	RollbackChangesOnNode(*types.Node, ...int)
	Dispense(DispenseOptions, *types.Resource1) (*types.Resource1, error)
}