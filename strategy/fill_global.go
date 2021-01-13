package strategy

import (
	"github.com/projecteru2/core/log"
)

// FillGlobalPlan try fill strategy and fallback to global strategy
func FillGlobalPlan(strategyInfo []Info, need, total, limit int) (map[string]int, error) {
	originStrategyInfo := make([]Info, len(strategyInfo))
	copy(originStrategyInfo, strategyInfo)

	deployMap, err := FillPlan(strategyInfo, need, total, limit)
	if err == nil {
		return deployMap, nil
	}
	log.Infof("[FillGlobalPlan] fill plan failed, try global fill: %+v", err)
	return GlobalPlan(originStrategyInfo, need, total, limit)
}
