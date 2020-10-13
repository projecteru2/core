package strategy

import (
	"sort"

	"github.com/pkg/errors"
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

type startegyFunc = func(_ []types.StrategyInfo, need, total, limit int, _ types.ResourceType) (map[string]*types.DeployInfo, error)

func scoreSort(strategyInfo []types.StrategyInfo, byResource types.ResourceType) []types.StrategyInfo {
	sort.Slice(strategyInfo, func(i, j int) bool {
		return strategyInfo[i].GetResourceUsage(byResource) < strategyInfo[j].GetResourceUsage(byResource)
	})
	return strategyInfo
}

// Deploy .
func Deploy(opts *types.DeployOptions, strategyInfos []types.StrategyInfo, total int, resourceTypes types.ResourceType) (map[string]*types.DeployInfo, error) {
	deployMethod, ok := Plans[opts.DeployStrategy]
	if !ok {
		return nil, errors.WithStack(types.ErrBadDeployStrategy)
	}

	return deployMethod(strategyInfos, opts.Count, total, opts.NodesLimit, resourceTypes)
}
