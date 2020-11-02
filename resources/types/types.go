package types

import (
	"github.com/projecteru2/core/types"
)

// SchedulerV2 .
type SchedulerV2 func([]types.NodeInfo) (ResourcePlans, int, error)

// DispenseOptions .
type DispenseOptions struct {
	*types.Node
	ExistingInstances  []*types.Container
	Index              int
	HardVolumeBindings types.VolumeBindings
}

// ResourceRequirement .
type ResourceRequirement interface {
	Type() types.ResourceType
	Validate() error
	MakeScheduler() SchedulerV2
	Rate(types.Node) float64
}

// ResourcePlans .
type ResourcePlans interface {
	Type() types.ResourceType
	Capacity() map[string]int
	ApplyChangesOnNode(*types.Node, ...int)
	RollbackChangesOnNode(*types.Node, ...int)
	Dispense(DispenseOptions, *types.Resources) error
}

// ResourceRequirements .
type ResourceRequirements [3]ResourceRequirement
