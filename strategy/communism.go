package strategy

import (
	"container/heap"
	"fmt"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/types"
)

// CommunismPlan 吃我一记共产主义大锅饭
// 部署完 N 个后全局尽可能平均

type infoHeap []Info

func (h infoHeap) Len() int {
	return len(h)
}

func (h infoHeap) Less(i, j int) bool {
	return h[i].Count < h[j].Count || (h[i].Count == h[j].Count && h[i].Capacity > h[j].Capacity)
}

func (h infoHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *infoHeap) Push(x interface{}) {
	*h = append(*h, x.(Info))
}

func (h *infoHeap) Pop() interface{} {
	length := len(*h)
	x := (*h)[length-1]
	*h = (*h)[0 : length-1]
	return x
}

func newInfoHeap(infos []Info) heap.Interface {
	dup := make(infoHeap, len(infos))
	copy(dup, infos)
	return &dup
}

func CommunismPlan(infos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	if total < need {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("need: %d, vol: %d", need, total)))
	}

	deploy := map[string]int{}
	iHeap := newInfoHeap(infos)
	heap.Init(iHeap)
	for {
		if iHeap.Len() == 0 {
			return nil, errors.WithStack(types.ErrInsufficientRes)
		}
		info := heap.Pop(iHeap).(Info)
		if info.Capacity == 0 {
			continue
		}
		deploy[info.Nodename]++
		need--
		if need == 0 {
			return deploy, nil
		}
		if limit > 0 && info.Count >= limit {
			continue
		}
		info.Count++
		info.Capacity--
		heap.Push(iHeap, info)
	}
}
