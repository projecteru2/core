package strategy

import (
	"container/heap"
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

type infoHeapForGlobalStrategy []Info

func (r infoHeapForGlobalStrategy) Len() int {
	return len(r)
}

func (r infoHeapForGlobalStrategy) Less(i, j int) bool {
	return (r[i].Usage + r[i].Rate) < (r[j].Usage + r[j].Rate)
}

func (r infoHeapForGlobalStrategy) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

func (r *infoHeapForGlobalStrategy) Push(x any) {
	*r = append(*r, x.(Info))
}

func (r *infoHeapForGlobalStrategy) Pop() any {
	old := *r
	n := len(old)
	x := old[n-1]
	*r = old[:n-1]
	return x
}

// GlobalPlan spreads need workloads to keep Usage+Rate as even as possible across nodes.
func GlobalPlan(ctx context.Context, infos []Info, need, total, _ int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}
	deployMap := map[string]int{}

	h := &infoHeapForGlobalStrategy{}
	for _, info := range infos {
		if info.Capacity > 0 {
			h.Push(info)
		}
	}
	heap.Init(h)

	for i := range need {
		if h.Len() == 0 {
			return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, i)
		}
		infoWithMinUsage := heap.Pop(h).(Info)
		deployMap[infoWithMinUsage.Nodename]++
		infoWithMinUsage.Usage += infoWithMinUsage.Rate
		infoWithMinUsage.Capacity--

		if infoWithMinUsage.Capacity > 0 {
			heap.Push(h, infoWithMinUsage)
		}
	}

	log.WithFunc("strategy.GlobalPlan").Debugf(ctx, "strategyInfos: %+v", infos)
	return deployMap, nil
}
