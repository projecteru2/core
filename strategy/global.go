package strategy

import (
	"container/heap"
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// GlobalPlan spreads need workloads to keep Usage+Rate as even as possible across nodes.
func GlobalPlan(ctx context.Context, infos []Info, need, total, _ int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}
	deployMap := map[string]int{}

	h := newInfoHeap(
		infos,
		func(a, b Info) bool { return (a.Usage + a.Rate) < (b.Usage + b.Rate) },
		func(info Info) bool { return info.Capacity > 0 },
	)

	for i := range need {
		if h.Len() == 0 {
			return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, i)
		}
		infoWithMinUsage := heap.Pop(h).(Info)
		deployMap[infoWithMinUsage.Nodename]++
		infoWithMinUsage.Usage += infoWithMinUsage.Rate
		infoWithMinUsage.Capacity--
		heap.Push(h, infoWithMinUsage)
	}

	log.WithFunc("strategy.GlobalPlan").Debugf(ctx, "strategyInfos: %+v", infos)
	return deployMap, nil
}
