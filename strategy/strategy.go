package strategy

import (
	"math"
	"sort"

	"github.com/pkg/errors"
	schedulerv2 "github.com/projecteru2/core/scheduler/v2"
	"github.com/projecteru2/core/types"
)

const (
	// Auto .
	Auto = "AUTO"
	// Fill .
	Fill = "FILL"
	// Each .
	Each = "EACH"
	// Global .
	Global = "GLOBAL"
)

var Plans = map[string]startegyFunc{
	Auto:   CommunismPlan,
	Fill:   FillPlan,
	Each:   AveragePlan,
	Global: GlobalPlan,
}

type startegyFunc = func(_ []StrategyInfo, need, total, limit int, _ types.ResourceType) (map[string]*types.DeployInfo, error)

func scoreSort(strategyInfo []StrategyInfo, byResource types.ResourceType) []StrategyInfo {
	sort.Slice(strategyInfo, func(i, j int) bool {
		return strategyInfo[i].GetResourceUsage(byResource) < strategyInfo[j].GetResourceUsage(byResource)
	})
	return strategyInfo
}

// Deploy .
func Deploy(opts *types.DeployOptions, strategyInfos []StrategyInfo, total int, resourceTypes types.ResourceType) (map[string]*types.DeployInfo, error) {
	deployMethod, ok := Plans[opts.DeployStrategy]
	if !ok {
		return nil, errors.WithStack(types.ErrBadDeployStrategy)
	}

	return deployMethod(strategyInfos, opts.Count, total, opts.NodesLimit, resourceTypes)
}

// StrategyInfo .
type StrategyInfo struct {
	Nodename string

	Usages map[types.ResourceType]float64
	Rates  map[types.ResourceType]float64

	Capacity int
	Count    int
}

// NewStrategyInfos .
func NewStrategyInfos(apps schedulerv2.ResourceApplications, nodeMap map[string]*types.Node, planMap map[types.ResourceType]schedulerv2.ResourcePlans) (strategyInfos []StrategyInfo) {
	for nodeName, node := range nodeMap {
		rates := make(map[types.ResourceType]float64)
		for _, app := range apps {
			rates[app.Type()] = app.Rate(*node)
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
func (s *StrategyInfo) GetResourceUsage(resource types.ResourceType) (usage float64) {
	for _, resourceType := range types.AllResourceTypes {
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
func (s *StrategyInfo) GetResourceRate(resource types.ResourceType) (rate float64) {
	for _, resourceType := range types.AllResourceTypes {
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
