package schedule

import (
	"cmp"
	"container/heap"
	"slices"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
)

type cpuCore struct {
	ID     string
	pieces int
}

func (c cpuCore) Less(c1 *cpuCore) bool {
	if c.pieces == c1.pieces {
		return c.ID < c1.ID
	}
	return c.pieces < c1.pieces
}

type cpuCoreHeap []*cpuCore

func (c cpuCoreHeap) Len() int {
	return len(c)
}

func (c cpuCoreHeap) Less(i, j int) bool {
	return !c[i].Less(c[j])
}

func (c cpuCoreHeap) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func (c *cpuCoreHeap) Push(x any) {
	*c = append(*c, x.(*cpuCore))
}

func (c *cpuCoreHeap) Pop() any {
	old := *c
	n := len(old)
	x := old[n-1]
	*c = old[:n-1]
	return x
}

type host struct {
	shareBase        int
	maxFragmentCores int
	fullCores        []*cpuCore
	fragmentCores    []*cpuCore
	affinity         bool
}

func newHost(cpuMap types.CPUMap, shareBase, maxFragmentCores int) *host {
	h := &host{
		shareBase:        shareBase,
		maxFragmentCores: maxFragmentCores,
		fullCores:        []*cpuCore{},
		fragmentCores:    []*cpuCore{},
	}

	for cpu, pieces := range cpuMap {
		if pieces >= shareBase && pieces%shareBase == 0 {
			h.fullCores = append(h.fullCores, &cpuCore{ID: cpu, pieces: pieces})
		} else if pieces > 0 {
			h.fragmentCores = append(h.fragmentCores, &cpuCore{ID: cpu, pieces: pieces})
		}
	}

	// busier cores go first so idle cores stay whole
	byLoad := func(a, b *cpuCore) int { return cmp.Or(cmp.Compare(a.pieces, b.pieces), cmp.Compare(a.ID, b.ID)) }
	slices.SortStableFunc(h.fullCores, byLoad)
	slices.SortStableFunc(h.fragmentCores, byLoad)

	return h
}

func (h *host) getCPUPlans(cpuRequest float64) []types.CPUMap {
	piecesRequest := int(cpuRequest * float64(h.shareBase))
	full := piecesRequest / h.shareBase
	fragment := piecesRequest % h.shareBase

	maxFragmentCores := len(h.fullCores) + len(h.fragmentCores) - full
	if h.maxFragmentCores == -1 || h.maxFragmentCores > maxFragmentCores {
		h.maxFragmentCores = maxFragmentCores
	}

	if fragment == 0 {
		return h.getFullCPUPlans(h.fullCores, full)
	}

	if full == 0 {
		diff := max(h.maxFragmentCores-len(h.fragmentCores), 0)
		h.fragmentCores = append(h.fragmentCores, h.fullCores[:diff]...)
		h.fullCores = h.fullCores[diff:]
		return h.getFragmentCPUPlans(h.fragmentCores, fragment)
	}

	fragmentCapacityMap := map[string]int{}
	totalFragmentCapacity := 0
	bestCPUPlans := [2][]types.CPUMap{h.getFullCPUPlans(h.fullCores, full), h.getFragmentCPUPlans(h.fragmentCores, fragment)}
	bestCapacity := min(len(bestCPUPlans[0]), len(bestCPUPlans[1]))

	for _, core := range h.fullCores {
		fragmentCapacityMap[core.ID] = core.pieces / fragment
	}

	for _, core := range h.fragmentCores {
		fragmentCapacityMap[core.ID] = core.pieces / fragment
		totalFragmentCapacity += fragmentCapacityMap[core.ID]
	}

	for len(h.fragmentCores) < h.maxFragmentCores {
		newFragmentCore := h.fullCores[0]
		h.fragmentCores = append(h.fragmentCores, newFragmentCore)
		h.fullCores = h.fullCores[1:]
		totalFragmentCapacity += fragmentCapacityMap[newFragmentCore.ID]

		fullCPUPlans := h.getFullCPUPlans(h.fullCores, full)
		capacity := min(len(fullCPUPlans), totalFragmentCapacity)
		if capacity > bestCapacity {
			bestCPUPlans[0] = fullCPUPlans
			bestCPUPlans[1] = h.getFragmentCPUPlans(h.fragmentCores, fragment)
			bestCapacity = capacity
		}
	}

	cpuPlans := []types.CPUMap{}
	for i := range bestCapacity {
		fullCPUPlans := bestCPUPlans[0]
		fragmentCPUPlans := bestCPUPlans[1]

		cpuMap := types.CPUMap{}
		cpuMap.Add(fullCPUPlans[i])
		cpuMap.Add(fragmentCPUPlans[i])

		cpuPlans = append(cpuPlans, cpuMap)
	}

	return cpuPlans
}

func (h *host) getFullCPUPlans(cores []*cpuCore, full int) []types.CPUMap {
	if h.affinity {
		return h.getFullCPUPlansWithAffinity(cores, full)
	}

	result := []types.CPUMap{}
	cpuHeap := &cpuCoreHeap{}
	indexMap := map[string]int{}
	for i, core := range cores {
		indexMap[core.ID] = i
		cpuHeap.Push(&cpuCore{ID: core.ID, pieces: core.pieces})
	}
	heap.Init(cpuHeap)

	for cpuHeap.Len() >= full {
		plan := types.CPUMap{}
		resourcesToPush := []*cpuCore{}

		for range full {
			core := heap.Pop(cpuHeap).(*cpuCore)
			plan[core.ID] = h.shareBase

			core.pieces -= h.shareBase
			if core.pieces > 0 {
				resourcesToPush = append(resourcesToPush, core)
			}
		}

		result = append(result, plan)
		for _, core := range resourcesToPush {
			heap.Push(cpuHeap, core)
		}
	}

	// restore the pre-heap core priority across the produced plans
	sumOfIDs := func(c types.CPUMap) int {
		sum := 0
		for ID := range c {
			sum += indexMap[ID]
		}
		return sum
	}

	slices.SortFunc(result, func(a, b types.CPUMap) int { return cmp.Compare(sumOfIDs(a), sumOfIDs(b)) })

	return result
}

func (h *host) getFullCPUPlansWithAffinity(cores []*cpuCore, full int) []types.CPUMap {
	result := []types.CPUMap{}

	for len(cores) >= full {
		count := len(cores) / full
		tempCores := []*cpuCore{}
		for i := range count {
			cpuMap := types.CPUMap{}
			for j := i * full; j < i*full+full; j++ {
				cpuMap[cores[j].ID] = h.shareBase

				remainingPieces := cores[j].pieces - h.shareBase
				if remainingPieces > 0 {
					tempCores = append(tempCores, &cpuCore{ID: cores[j].ID, pieces: remainingPieces})
				}
			}
			result = append(result, cpuMap)
		}

		cores = append(tempCores, cores[len(cores)/full*full:]...)
	}

	return result
}

func (h *host) getFragmentCPUPlans(cores []*cpuCore, fragment int) []types.CPUMap {
	result := []types.CPUMap{}
	for _, core := range cores {
		for range core.pieces / fragment {
			result = append(result, types.CPUMap{core.ID: fragment})
		}
	}
	return result
}

func GetCPUPlans(resourceInfo *types.NodeResourceInfo, originCPUMap types.CPUMap, shareBase, maxFragmentCores int, req *types.WorkloadResourceRequest) []*types.CPUPlan {
	cpuPlans := []*types.CPUPlan{}
	availableResource := resourceInfo.GetAvailableResource()

	numaCPUMap := map[string]types.CPUMap{}
	for cpuID, numaNodeID := range resourceInfo.Capacity.NUMA {
		if _, ok := numaCPUMap[numaNodeID]; !ok {
			numaCPUMap[numaNodeID] = types.CPUMap{}
		}
		numaCPUMap[numaNodeID][cpuID] = availableResource.CPUMap[cpuID]
	}

	for numaNodeID, cpuMap := range numaCPUMap {
		numaCPUPlans := doGetCPUPlans(originCPUMap, cpuMap, availableResource.NUMAMemory[numaNodeID], shareBase, maxFragmentCores, req.CPURequest, req.MemRequest)
		for _, workloadCPUMap := range numaCPUPlans {
			cpuPlans = append(cpuPlans, &types.CPUPlan{
				NUMANode: numaNodeID,
				CPUMap:   workloadCPUMap,
			})
			availableResource.Sub(&types.NodeResource{
				CPU:        req.CPURequest,
				CPUMap:     workloadCPUMap,
				Memory:     req.MemRequest,
				NUMAMemory: types.NUMAMemory{numaNodeID: req.MemRequest},
			})
		}
	}

	crossNUMACPUPlans := doGetCPUPlans(originCPUMap, availableResource.CPUMap, availableResource.Memory, shareBase, maxFragmentCores, req.CPURequest, req.MemRequest)
	for _, workloadCPUMap := range crossNUMACPUPlans {
		cpuPlans = append(cpuPlans, &types.CPUPlan{
			CPUMap: workloadCPUMap,
		})
	}

	return cpuPlans
}

func doGetCPUPlans(originCPUMap, availableCPUMap types.CPUMap, availableMemory int64, shareBase, maxFragmentCores int, cpuRequest float64, memoryRequest int64) []types.CPUMap {
	h := newHost(availableCPUMap, shareBase, maxFragmentCores)

	if len(originCPUMap) > 0 {
		originH := newHost(originCPUMap, shareBase, maxFragmentCores)
		reorderByAffinity(originH, h)
	}

	cpuPlans := h.getCPUPlans(cpuRequest)
	if memoryRequest > 0 {
		memoryCapacity := int(availableMemory / memoryRequest)
		if memoryCapacity < len(cpuPlans) {
			cpuPlans = cpuPlans[:memoryCapacity]
		}
	}
	return cpuPlans
}

// reorderByAffinity keeps the cores the workload already holds at the front of newH.
func reorderByAffinity(oldH, newH *host) {
	oldFull := map[string]int{}
	oldFragment := map[string]int{}

	for i, core := range oldH.fullCores {
		oldFull[core.ID] = i + 1
	}
	for i, core := range oldH.fragmentCores {
		oldFragment[core.ID] = i + 1
	}

	sortFunc := func(orderMap map[string]int) func(a, b *cpuCore) int {
		return func(a, b *cpuCore) int {
			idxA, idxB := orderMap[a.ID], orderMap[b.ID]
			if idxA == 0 && idxB == 0 {
				return 0
			}
			if idxA == 0 || idxB == 0 {
				return cmp.Compare(idxB, idxA)
			}
			return cmp.Compare(idxA, idxB)
		}
	}

	slices.SortStableFunc(newH.fullCores, sortFunc(oldFull))
	slices.SortStableFunc(newH.fragmentCores, sortFunc(oldFragment))
	newH.affinity = true
}
