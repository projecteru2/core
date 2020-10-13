package types

import (
	"math"
)

// ResourceType .
type ResourceType int

const (
	// ResourceCPU .
	ResourceCPU ResourceType = 1 << iota
	// ResourceCPUBind .
	ResourceCPUBind
	// ResourceMemory .
	ResourceMemory
	// ResourceVolume .
	ResourceVolume
	// ResourceScheduledVolume .
	ResourceScheduledVolume
	// ResourceStorage .
	ResourceStorage
)

var (
	// ResourceAll .
	ResourceAll = ResourceStorage | ResourceMemory | ResourceCPU | ResourceVolume
	// AllResourceTypes .
	AllResourceTypes = [...]ResourceType{ResourceCPU, ResourceMemory, ResourceVolume, ResourceStorage}
)

// SchedulerV2 .
type SchedulerV2 func([]NodeInfo) (ResourcePlans, int, error)

// ResourceRequest .
type ResourceRequest interface {
	Type() ResourceType
	DeployValidate() error
	MakeScheduler() SchedulerV2
	Rate(Node) float64
}

// ResourcePlans .
type ResourcePlans interface {
	Type() ResourceType
	Capacity() map[string]int
	ApplyChangesOnNode(*Node, ...int)
	RollbackChangesOnNode(*Node, ...int)
	Dispense(DispenseOptions, *Resources) error
}

// DispenseOptions .
type DispenseOptions struct {
	*Node
	ExistingInstances  []*Container
	Index              int
	HardVolumeBindings VolumeBindings
}

// StrategyInfo .
type StrategyInfo struct {
	Nodename string

	Usages map[ResourceType]float64
	Rates  map[ResourceType]float64

	Capacity int
	Count    int
}

// DeployInfo .
type DeployInfo struct {
	Deploy int
}

// NewStrategyInfos .
func NewStrategyInfos(opts *DeployOptions, nodeMap map[string]*Node, planMap map[ResourceType]ResourcePlans) (strategyInfos []StrategyInfo) {
	for nodeName, node := range nodeMap {
		rates := make(map[ResourceType]float64)
		for _, req := range opts.ResourceRequests {
			rates[req.Type()] = req.Rate(*node)
		}

		capacity := math.MaxInt32
		for _, plan := range planMap {
			if plan.Capacity()[nodeName] < capacity {
				capacity = plan.Capacity()[nodeName]
			}
		}
		if capacity <= 0 {
			continue
		}

		strategyInfos = append(strategyInfos, StrategyInfo{
			Nodename: nodeName,
			Rates:    rates,
			Usages:   node.ResourceUsages(),
			Capacity: capacity,
		})
	}
	return
}

// GetResourceUsage .
func (s *StrategyInfo) GetResourceUsage(resource ResourceType) (usage float64) {
	for _, resourceType := range AllResourceTypes {
		if resourceType&resource != 0 {
			for t, u := range s.Usages {
				if t&resourceType != 0 {
					usage += u
				}
			}
		}
	}
	return
}

// GetResourceRate .
func (s *StrategyInfo) GetResourceRate(resource ResourceType) (rate float64) {
	for _, resourceType := range AllResourceTypes {
		if resourceType&resource != 0 {
			for t, r := range s.Rates {
				if t&resourceType != 0 {
					rate += r
				}
			}
		}
	}
	return
}
