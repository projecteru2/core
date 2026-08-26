package strategy

import (
	"container/heap"
)

type infoLess func(a, b Info) bool

type infoAdmit func(info Info) bool

type infoHeap struct {
	infos []Info
	less  infoLess
	admit infoAdmit
}

func newInfoHeap(infos []Info, less infoLess, admit infoAdmit) *infoHeap {
	h := &infoHeap{infos: make([]Info, 0, len(infos)), less: less, admit: admit}
	for _, info := range infos {
		h.Push(info)
	}
	heap.Init(h)
	return h
}

func (h *infoHeap) Len() int {
	return len(h.infos)
}

func (h *infoHeap) Less(i, j int) bool {
	return h.less(h.infos[i], h.infos[j])
}

func (h *infoHeap) Swap(i, j int) {
	h.infos[i], h.infos[j] = h.infos[j], h.infos[i]
}

func (h *infoHeap) Push(x any) {
	info := x.(Info)
	if !h.admit(info) {
		return
	}
	h.infos = append(h.infos, info)
}

func (h *infoHeap) Pop() any {
	length := len(h.infos)
	x := h.infos[length-1]
	h.infos = h.infos[:length-1]
	return x
}
