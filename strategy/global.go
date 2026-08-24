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
	strategyInfos := make([]Info, len(infos))
	copy(strategyInfos, infos)
	deployMap := map[string]int{}

	infoHeap := &infoHeapForGlobalStrategy{}
	for _, info := range strategyInfos {
		if info.Capacity > 0 {
			infoHeap.Push(info)
		}
	}
	heap.Init(infoHeap)

	for i := range need {
		if infoHeap.Len() == 0 {
			return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, i)
		}
		infoWithMinUsage := heap.Pop(infoHeap).(Info)
		deployMap[infoWithMinUsage.Nodename]++
		infoWithMinUsage.Usage += infoWithMinUsage.Rate
		infoWithMinUsage.Capacity--

		if infoWithMinUsage.Capacity > 0 {
			heap.Push(infoHeap, infoWithMinUsage)
		}
	}

	log.WithFunc("strategy.GlobalPlan").Debugf(ctx, "strategyInfos: %+v", strategyInfos)
	return deployMap, nil
}
