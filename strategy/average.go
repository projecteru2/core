package strategy

import (
	"context"
	"fmt"
	"sort"

	"github.com/pkg/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// AveragePlan deploy workload each node
// 容量够的机器每一台部署 N 个
// need 是每台机器所需总量，limit 是限制节点数, 保证本轮增量部署 need*limit 个实例
// limit = 0 即对所有节点部署
func AveragePlan(ctx context.Context, infos []Info, need, total, limit int) (map[string]int, error) {
	log.Debugf(ctx, "[AveragePlan] need %d limit %d infos %v", need, limit, infos)
	scheduleInfosLength := len(infos)
	if limit == 0 {
		limit = scheduleInfosLength
	}
	if scheduleInfosLength < limit {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d < limit, cannot alloc an average node plan", scheduleInfosLength)))
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Capacity > infos[j].Capacity })
	p := sort.Search(scheduleInfosLength, func(i int) bool { return infos[i].Capacity < need })
	if p == 0 {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientCap, "insufficient nodes, at least 1 needed"))
	}
	if p < limit {
		return nil, types.NewDetailedErr(types.ErrInsufficientRes, fmt.Sprintf("not enough nodes with capacity of %d, require %d nodes", need, limit))
	}
	deployMap := map[string]int{}
	for _, strategyInfo := range infos[:limit] {
		deployMap[strategyInfo.Nodename] += need
	}

	return deployMap, nil
}
