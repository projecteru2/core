package strategy

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// AveragePlan deploy workload each node
// 容量够的机器每一台部署 N 个
// need 是每台机器所需总量，limit 是限制节点数
func AveragePlan(infos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	log.Debugf("[AveragePlan] need %d limit %d", need, limit)
	scheduleInfosLength := len(infos)
	if scheduleInfosLength < limit {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d < limit, cannot alloc an average node plan", scheduleInfosLength)))
	}
	strategyInfos := make([]Info, scheduleInfosLength)
	copy(strategyInfos, infos)
	sort.Slice(strategyInfos, func(i, j int) bool { return strategyInfos[i].Capacity < strategyInfos[j].Capacity })
	p := sort.Search(scheduleInfosLength, func(i int) bool { return strategyInfos[i].Capacity >= need })
	if p == scheduleInfosLength {
		return nil, errors.WithStack(types.ErrInsufficientCap)
	}
	strategyInfos = scoreSort(strategyInfos[p:], resourceType)
	if limit > 0 {
		strategyInfos = strategyInfos[:limit]
	}
	deployMap := map[string]int{}
	for i, strategyInfo := range strategyInfos {
		strategyInfos[i].Capacity -= need
		deployMap[strategyInfo.Nodename] += need
	}

	log.Debugf("[AveragePlan] resource: %v, strategyInfos: %v", resourceType, strategyInfos)
	return deployMap, nil
}
