package strategy

import (
	"container/heap"
	"fmt"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/types"
)

type infoHeap struct {
	infos []Info
	limit int
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

func (h *infoHeap) Push(x interface{}) {
	info := x.(Info)
	if info.Capacity == 0 || (h.limit > 0 && info.Count >= h.limit) {
		return
	}
	h.infos = append(h.infos, info)
}

func (h *infoHeap) Pop() interface{} {
	length := len(h.infos)
	x := h.infos[length-1]
	h.infos = h.infos[0 : length-1]
	return x
}

func newInfoHeap(infos []Info, limit int) heap.Interface {
	dup := infoHeap{
		infos: []Info{},
		limit: limit,
	}
	for _, info := range infos {
		if info.Capacity == 0 || (limit > 0 && info.Count >= limit) {
			continue
		}
		dup.infos = append(dup.infos, info)
	}
	return &dup
}

// CommunismPlan 吃我一记共产主义大锅饭
// 部署完 N 个后全局尽可能平均
func CommunismPlan(infos []Info, need, total, limit int, resourceType types.ResourceType) (map[string]int, error) {
	if total < need {
		return nil, errors.WithStack(types.NewDetailedErr(types.ErrInsufficientRes,
			fmt.Sprintf("need: %d, vol: %d", need, total)))
	}

	deploy := map[string]int{}
	iHeap := newInfoHeap(infos, limit)
	heap.Init(iHeap)
	for {
		if iHeap.Len() == 0 {
			return nil, errors.WithStack(types.ErrInsufficientRes)
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
