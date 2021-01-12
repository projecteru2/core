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
// need 是每台机器所需总量，limit 是限制节点数, 保证本轮增量部署 need*limit 个实例
func AveragePlan(infos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	log.Debugf("[AveragePlan] need %d limit %d infos %v", need, limit, infos)
	scheduleInfosLength := len(infos)
	if scheduleInfosLength < limit {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d < limit, cannot alloc an average node plan", scheduleInfosLength)))
	}
	strategyInfos := make([]Info, scheduleInfosLength)
	copy(strategyInfos, infos)
	sort.Slice(strategyInfos, func(i, j int) bool { return strategyInfos[i].Capacity > strategyInfos[j].Capacity })
	p := sort.Search(scheduleInfosLength, func(i int) bool { return strategyInfos[i].Capacity < need })
	if p == 0 {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientCap, "insufficient nodes, at least 1 needed"))
	}
	if limit > 0 && limit > p {
		return nil, types.NewDetailedErr(types.ErrInsufficientRes, fmt.Sprintf("insufficient nodes, %d more needed", limit-p))
	}
	deployMap := map[string]int{}
	n := len(strategyInfos)
	if limit > 0 {
		n = limit
	}
	for _, strategyInfo := range strategyInfos[:n] {
		deployMap[strategyInfo.Nodename] += need
	}

	return deployMap, nil
}
