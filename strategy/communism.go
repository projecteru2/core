package strategy

import (
	"container/heap"
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

// CommunismPlan spreads need workloads so the global per-node count ends as even as possible.
func CommunismPlan(_ context.Context, infos []Info, need, total, limit int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}

	deploy := map[string]int{}
	iHeap := newInfoHeap(
		infos,
		func(a, b Info) bool {
			return a.Count < b.Count || (a.Count == b.Count && a.Capacity > b.Capacity)
		},
		func(info Info) bool {
			return info.Capacity != 0 && (limit <= 0 || info.Count < limit)
		},
	)
	for {
		if iHeap.Len() == 0 {
			return nil, errors.Wrapf(types.ErrInsufficientResource, "reached nodelimit, a node can host at most %d instances", limit)
		}
		info := heap.Pop(iHeap).(Info)
		deploy[info.Nodename]++
		need--
		if need == 0 {
			return deploy, nil
		}
		info.Count++
		info.Capacity--
		if iHeap.admit(info) {
			heap.Push(iHeap, info)
		}
	}
}
