package strategy

import (
	"fmt"
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// FillPlan deploy workload each node
// 根据之前部署的策略每一台补充到 N 个，已经超过 N 个的节点视为已满足
// need 是每台上限, limit 是限制节点数, 保证最终状态至少有 limit*need 个实例
// limit = 0 代表对所有节点进行填充
func FillPlan(infos []Info, need, _, limit int) (_ map[string]int, err error) {
	log.Debugf("[FillPlan] need %d limit %d infos %+v", need, limit, infos)
	scheduleInfosLength := len(infos)
	if limit == 0 {
		limit = scheduleInfosLength
	}
	if scheduleInfosLength < limit {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("node len %d cannot alloc a fill node plan", scheduleInfosLength)))
	}
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].Count == infos[j].Count {
			return infos[i].Capacity > infos[j].Capacity
		}
		return infos[i].Count > infos[j].Count
	})
	deployMap, toDeploy := make(map[string]int), 0
	for _, info := range infos {
		if info.Count+info.Capacity >= need {
			deployMap[info.Nodename] += utils.Max(need-info.Count, 0)
			toDeploy += deployMap[info.Nodename]
			limit--
			if limit == 0 {
				if toDeploy == 0 {
					err = errors.WithStack(types.ErrAlreadyFilled)
				}
				return deployMap, err
			}
		}
	}
	return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
		fmt.Sprintf("insufficient nodes to fill %d, %d more nodes needed", need, limit)))
}
