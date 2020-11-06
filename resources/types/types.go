package types

import (
	"github.com/projecteru2/core/types"
)

const supported = 3

// ResourceRequirements .
type ResourceRequirements [supported]ResourceRequirement

// ResourceRequirement .
type ResourceRequirement interface {
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
	Dispense(DispenseOptions) (*types.Resource, error)
}
