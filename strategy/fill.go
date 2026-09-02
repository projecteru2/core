package strategy

import (
	"cmp"
	"context"
	"slices"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// FillPlan tops every node up to need workloads; need is a per-node ceiling, limit 0 means every node.
func FillPlan(_ context.Context, infos []Info, need, _, limit int) (_ map[string]int, err error) {
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
	deployMap, toDeploy, remain := make(map[string]int), 0, limit
	for _, info := range infos {
		if info.Count+info.Capacity >= need {
			deploy := max(need-info.Count, 0)
			deployMap[info.Nodename] = deploy
			toDeploy += deploy
			remain--
			if remain == 0 {
				if toDeploy == 0 {
					err = types.ErrAlreadyFilled
				}
				return deployMap, err
			}
		}
	}
	return nil, errors.Wrapf(types.ErrInsufficientResource, "not enough nodes that can fill up to %d instances, require %d nodes", need, limit)
}
