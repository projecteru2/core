package strategy

import (
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

func FillGlobalPlan(strategyInfo []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	originStrategyInfo := make([]Info, len(strategyInfo))
	copy(originStrategyInfo, strategyInfo)

	deployMap, err := FillPlan(strategyInfo, need, total, limit, resourceType)
	if err == nil {
		return deployMap, nil
	}
	log.Info("[FillGlobalPlan] fill plan failed, try global fill: %+v", err)
	return GlobalPlan(originStrategyInfo, need, total, limit, resourceType)
}
