package strategy

import (
	"context"
	"sort"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// FillPlan tops every node up to need workloads and skips nodes already at or above need.
// need is the per-node ceiling, not an increment; limit 0 means every node
func FillPlan(ctx context.Context, infos []Info, need, _, limit int) (_ map[string]int, err error) {
	log.WithFunc("strategy.FillPlan").Debugf(ctx, "need %d limit %d infos %+v", need, limit, infos)
	scheduleInfosLength := len(infos)
	if limit == 0 {
		limit = scheduleInfosLength
	}
	if scheduleInfosLength < limit {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "node len %d cannot alloc a fill node plan", scheduleInfosLength)
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
					err = types.ErrAlreadyFilled
				}
				return deployMap, err
			}
		}
	}
	return nil, errors.Wrapf(types.ErrInsufficientResource, "not enough nodes that can fill up to %d instances, require %d nodes", need, limit)
}
