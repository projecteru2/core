package strategy

import (
	"math"
	"sort"

	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
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
	// FillGlobal .
	FillGlobal = "FILLGLOBAL"
	// Dummy for calculate capacity
	Dummy = "DUMMY"
)

var Plans = map[string]startegyFunc{
	Auto:       CommunismPlan,
	Fill:       FillPlan,
	Each:       AveragePlan,
	Global:     GlobalPlan,
	FillGlobal: FillGlobalPlan,
}

type startegyFunc = func(_ []Info, need, total, limit int, _ types.ResourceType) (map[string]int, error)

func scoreSort(strategyInfo []Info, byResource types.ResourceType) []Info {
	sort.Slice(strategyInfo, func(i, j int) bool {
		return strategyInfo[i].GetResourceUsage(byResource) < strategyInfo[j].GetResourceUsage(byResource)
	})
	return strategyInfo
}

// Deploy .
func Deploy(opts *types.DeployOptions, strategyInfos []Info, total int, resourceTypes types.ResourceType) (map[string]int, error) {
	deployMethod, ok := Plans[opts.DeployStrategy]
	if !ok {
		return nil, errors.WithStack(types.ErrBadDeployStrategy)
	}

	return deployMethod(strategyInfos, opts.Count, total, opts.NodesLimit, resourceTypes)
}

// Info .
type Info struct {
	Nodename string

	Usages map[types.ResourceType]float64
	Rates  map[types.ResourceType]float64

	Capacity int
	Count    int
}

// NewInfos .
// TODO strange name, need to revise
func NewInfos(resourceRequests resourcetypes.ResourceRequests, nodeMap map[string]*types.Node, plans []resourcetypes.ResourcePlans) (strategyInfos []Info) {
	for nodename, node := range nodeMap {
		rates := make(map[types.ResourceType]float64)
		for _, resourceRequest := range resourceRequests {
			rates[resourceRequest.Type()] = resourceRequest.Rate(*node)
		}

		capacity := math.MaxInt64
		for _, plan := range plans {
			if plan.Capacity()[nodename] < capacity {
				capacity = plan.Capacity()[nodename]
			}
		}
		if capacity <= 0 {
			continue
		}

		strategyInfos = append(strategyInfos, Info{
			Nodename: nodename,
			Rates:    rates,
			Usages:   node.ResourceUsages(),
			Capacity: capacity,
		})
	}
	return
}

// GetResourceUsage .
func (s *Info) GetResourceUsage(resource types.ResourceType) (usage float64) {
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
func (s *Info) GetResourceRate(resource types.ResourceType) (rate float64) {
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
