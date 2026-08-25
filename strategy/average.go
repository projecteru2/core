package strategy

import (
	"cmp"
	"context"
	"slices"
	"sort"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// AveragePlan deploys need workloads onto each of limit nodes with enough capacity, adding need*limit instances.
// need is per node, not a total; limit 0 means every node
func AveragePlan(ctx context.Context, infos []Info, need, _, limit int) (map[string]int, error) {
	log.WithFunc("strategy.AveragePlan").Debugf(ctx, "need %d limit %d infos %+v", need, limit, infos)
	scheduleInfosLength := len(infos)
	if limit == 0 {
		limit = scheduleInfosLength
	}
	if scheduleInfosLength < limit {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "node len %d < limit, cannot alloc an average node plan", scheduleInfosLength)
	}
	slices.SortFunc(infos, func(a, b Info) int { return cmp.Compare(b.Capacity, a.Capacity) })
	p := sort.Search(scheduleInfosLength, func(i int) bool { return infos[i].Capacity < need })
	if p == 0 {
		return nil, errors.Wrap(types.ErrInsufficientCapacity, "insufficient nodes, at least 1 needed")
	}
	if p < limit {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "not enough nodes with capacity of %d, require %d nodes", need, limit)
	}
	deployMap := map[string]int{}
	for _, strategyInfo := range infos[:limit] {
		deployMap[strategyInfo.Nodename] += need
	}

	return deployMap, nil
}
