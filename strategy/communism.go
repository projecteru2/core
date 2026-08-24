package strategy

import (
	"container/heap"
	"context"

	"github.com/projecteru2/core/types"

	"github.com/cockroachdb/errors"
)

type infoHeap struct {
	infos []Info
	limit int
}

func newInfoHeap(infos []Info, limit int) *infoHeap {
	h := &infoHeap{limit: limit}
	for _, info := range infos {
		h.Push(info)
	}
	return h
}

func (h infoHeap) Len() int {
	return len(h.infos)
}

func (h infoHeap) Less(i, j int) bool {
	return h.infos[i].Count < h.infos[j].Count || (h.infos[i].Count == h.infos[j].Count && h.infos[i].Capacity > h.infos[j].Capacity)
}

func (h infoHeap) Swap(i, j int) {
	h.infos[i], h.infos[j] = h.infos[j], h.infos[i]
}

func (h *infoHeap) Push(x any) {
	info := x.(Info)
	if info.Capacity == 0 || (h.limit > 0 && info.Count >= h.limit) {
		return
	}
	h.infos = append(h.infos, info)
}

func (h *infoHeap) Pop() any {
	length := len(h.infos)
	x := h.infos[length-1]
	h.infos = h.infos[0 : length-1]
	return x
}

// CommunismPlan spreads need workloads so the global per-node count ends as even as possible.
func CommunismPlan(_ context.Context, infos []Info, need, total, limit int) (map[string]int, error) {
	if total < need {
		return nil, errors.Wrapf(types.ErrInsufficientResource, "need: %d, available: %d", need, total)
	}

	deploy := map[string]int{}
	iHeap := newInfoHeap(infos, limit)
	heap.Init(iHeap)
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
		heap.Push(iHeap, info)
	}
}
