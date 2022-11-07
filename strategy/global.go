package strategy

import (
	"container/heap"
	"context"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

type infoHeapForGlobalStrategy []Info

// Len .
func (r infoHeapForGlobalStrategy) Len() int {
	return len(r)
}

// Less .
func (r infoHeapForGlobalStrategy) Less(i, j int) bool {
	return (r[i].Usage + r[i].Rate) < (r[j].Usage + r[j].Rate)
}

// Swap .
func (r infoHeapForGlobalStrategy) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Push .
func (r *infoHeapForGlobalStrategy) Push(x interface{}) {
	*r = append(*r, x.(Info))
}

// Pop .
func (r *infoHeapForGlobalStrategy) Pop() interface{} {
	old := *r
	n := len(old)
	x := old[n-1]
	*r = old[:n-1]
	return x
}

// GlobalPlan 基于全局资源配额
// 尽量使得资源消耗平均
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

	for i := 0; i < need; i++ {
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

	// 这里 need 一定会为 0 出来，因为 volTotal 保证了一定大于 need
	// 这里并不需要再次排序了，理论上的排序是基于资源使用率得到的 Deploy 最终方案
	log.WithFunc("strategy.GlobalPlan").Debugf(ctx, "strategyInfos: %+v", strategyInfos)
	return deployMap, nil
}
