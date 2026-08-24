package strategy

import (
	"cmp"
	"context"
	"slices"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
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
	slices.SortFunc(infos, func(a, b Info) int {
		return cmp.Or(cmp.Compare(b.Count, a.Count), cmp.Compare(b.Capacity, a.Capacity))
	})
	deployMap, toDeploy := make(map[string]int), 0
	for _, info := range infos {
		if info.Count+info.Capacity >= need {
			deployMap[info.Nodename] += max(need-info.Count, 0)
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
