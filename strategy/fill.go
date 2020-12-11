package strategy

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// FillPlan deploy workload each node
// 根据之前部署的策略每一台补充到 N 个，超过 N 个忽略
// need 是每台上限, limit 是限制节点数
func FillPlan(strategyInfos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	log.Debugf("[FillPlan] need %d limit %d", need, limit)
	scheduleInfosLength := len(strategyInfos)
	if scheduleInfosLength < limit {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d cannot alloc a fill node plan", scheduleInfosLength)))
	}
	sort.Slice(strategyInfos, func(i, j int) bool { return strategyInfos[i].Count > strategyInfos[j].Count })
	p := sort.Search(scheduleInfosLength, func(i int) bool { return strategyInfos[i].Count < need })
	if p == scheduleInfosLength {
		return nil, errors.WithStack(types.ErrAlreadyFilled)
	}
	strategyInfos = scoreSort(strategyInfos[p:], resourceType)
	if limit > 0 && len(strategyInfos) > limit {
		strategyInfos = strategyInfos[:limit]
	}
	deployMap := make(map[string]int)
	for i, strategyInfo := range strategyInfos {
		diff := need - strategyInfos[i].Count
		if strategyInfos[i].Capacity < diff {
			return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
				fmt.Sprintf("node %s cannot alloc a fill node plan", strategyInfos[i].Nodename)))
		}
		strategyInfos[i].Capacity -= diff
		deployMap[strategyInfo.Nodename] += diff
	}
	log.Debugf("[FillPlan] resource: %v, strategyInfos: %v", resourceType, strategyInfos)
	return deployMap, nil
}
