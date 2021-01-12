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
func FillPlan(infos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	log.Debugf("[FillPlan] need %d limit %d infos", need, limit, infos)
	scheduleInfosLength := len(infos)
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
	deployMap := make(map[string]int)
	for _, info := range infos {
		if info.Count+info.Capacity >= need {
			deployMap[info.Nodename] += utils.Max(need-info.Count, 0)
			limit -= 1
			if limit == 0 {
				return deployMap, nil
			}
		}
	}
	if limit > 0 {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("insufficient nodes to fill %d, %d more nodes needed", need, limit)))
	}
	return deployMap, nil
}
