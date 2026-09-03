package schedule

import (
	"cmp"
	"container/heap"
	"slices"
	"sort"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
	"github.com/projecteru2/core/utils"
)

type cpuCore struct {
	ID     string
	pieces int
	index  int
}

type cpuCoreHeap []*cpuCore

func (c cpuCoreHeap) Len() int {
	return len(c)
}

func (c cpuCoreHeap) Less(i, j int) bool {
	return byLoad(c[i], c[j]) >= 0
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
		fullCores:        make([]*cpuCore, 0, len(cpuMap)),
		fragmentCores:    make([]*cpuCore, 0, len(cpuMap)),
	}

	for cpu, pieces := range cpuMap {
		if pieces >= shareBase && pieces%shareBase == 0 {
			h.fullCores = append(h.fullCores, &cpuCore{ID: cpu, pieces: pieces})
		} else if pieces > 0 {
			h.fragmentCores = append(h.fragmentCores, &cpuCore{ID: cpu, pieces: pieces})
		}
	}

	// busier cores go first so idle cores stay whole
	slices.SortStableFunc(h.fullCores, byLoad)
	slices.SortStableFunc(h.fragmentCores, byLoad)

	return h
}

func (h *host) getCPUPlans(cpuRequest float64) []types.CPUMap {
	full, fragment, maxFragmentCores := h.split(cpuRequest)
	switch {
	case full == 0 && fragment == 0:
		return nil
	case fragment == 0:
		return h.getFullCPUPlans(h.fullCores, full)
	case full == 0:
		return h.getFragmentCPUPlans(h.moveToFragment(maxFragmentCores), fragment)
	}

	bestMoved, bestCapacity := h.bestSplit(full, fragment, h.maxMoved(maxFragmentCores))
	fullCPUPlans := h.getFullCPUPlans(h.fullCores[bestMoved:], full)
	fragmentCPUPlans := h.getFragmentCPUPlans(append(slices.Clone(h.fragmentCores), h.fullCores[:bestMoved]...), fragment)
	cpuPlans := make([]types.CPUMap, 0, bestCapacity)
	for i := range bestCapacity {
		cpuMap := types.CPUMap{}
		cpuMap.Add(fullCPUPlans[i])
		cpuMap.Add(fragmentCPUPlans[i])
		cpuPlans = append(cpuPlans, cpuMap)
	}
	return cpuPlans
}

func (h *host) countCPUPlans(cpuRequest float64) int {
	full, fragment, maxFragmentCores := h.split(cpuRequest)
	switch {
	case full == 0 && fragment == 0:
		return 0
	case fragment == 0:
		return h.countFullCPUPlans(h.fullCores, full)
	case full == 0:
		return h.countFragmentCPUPlans(h.moveToFragment(maxFragmentCores), fragment)
	}

	_, bestCapacity := h.bestSplit(full, fragment, h.maxMoved(maxFragmentCores))
	return bestCapacity
}

func (h *host) split(cpuRequest float64) (full, fragment, maxFragmentCores int) {
	piecesRequest := int(cpuRequest * float64(h.shareBase))
	full, fragment = piecesRequest/h.shareBase, piecesRequest%h.shareBase

	maxFragmentCores = len(h.fullCores) + len(h.fragmentCores) - full
	if h.maxFragmentCores != -1 && h.maxFragmentCores < maxFragmentCores {
		maxFragmentCores = h.maxFragmentCores
	}
	return full, fragment, maxFragmentCores
}

func (h *host) maxMoved(maxFragmentCores int) int {
	return max(min(maxFragmentCores-len(h.fragmentCores), len(h.fullCores)), 0)
}

func (h *host) moveToFragment(maxFragmentCores int) []*cpuCore {
	diff := max(maxFragmentCores-len(h.fragmentCores), 0)
	h.fragmentCores = append(h.fragmentCores, h.fullCores[:diff]...)
	h.fullCores = h.fullCores[diff:]
	return h.fragmentCores
}

// bestSplit finds the first split with the largest capacity where the falling full-plan count meets the rising fragment count.
func (h *host) bestSplit(full, fragment, maxMoved int) (int, int) {
	fragmentCapacity := make([]int, maxMoved+1)
	for _, core := range h.fragmentCores {
		fragmentCapacity[0] += core.pieces / fragment
	}
	for moved := 1; moved <= maxMoved; moved++ {
		fragmentCapacity[moved] = fragmentCapacity[moved-1] + h.fullCores[moved-1].pieces/fragment
	}
	fullCapacity := func(moved int) int { return h.countFullCPUPlans(h.fullCores[moved:], full) }
	firstReaching := func(capacity int) int {
		at, _ := slices.BinarySearch(fragmentCapacity, capacity)
		return at
	}

	crossing := sort.Search(maxMoved+1, func(moved int) bool { return fragmentCapacity[moved] >= fullCapacity(moved) })
	if crossing > maxMoved {
		return firstReaching(fragmentCapacity[maxMoved]), fragmentCapacity[maxMoved]
	}
	capacity := fullCapacity(crossing)
	if crossing > 0 && fragmentCapacity[crossing-1] >= capacity {
		return firstReaching(fragmentCapacity[crossing-1]), fragmentCapacity[crossing-1]
	}
	return crossing, capacity
}

func (h *host) countFullCPUPlans(cores []*cpuCore, full int) int {
	count := 0
	h.eachFullCPUPlan(cores, full, func([]int) { count++ })
	return count
}

func (h *host) getFullCPUPlans(cores []*cpuCore, full int) []types.CPUMap {
	type ranked struct {
		plan types.CPUMap
		rank int
	}
	plans := []ranked{}
	h.eachFullCPUPlan(cores, full, func(picked []int) {
		plan, rank := types.CPUMap{}, 0
		for _, i := range picked {
			plan[cores[i].ID] = h.shareBase
			rank += i
		}
		plans = append(plans, ranked{plan, rank})
	})
	if !h.affinity {
		// restore the pre-heap core priority across the produced plans
		slices.SortFunc(plans, func(a, b ranked) int { return cmp.Compare(a.rank, b.rank) })
	}
	return utils.Map(plans, func(r ranked) types.CPUMap { return r.plan })
}

// eachFullCPUPlan visits every whole-core plan as the indexes of its cores; with affinity the cores go in order, otherwise the busiest first.
func (h *host) eachFullCPUPlan(cores []*cpuCore, full int, visit func(picked []int)) {
	if h.affinity {
		pieces := make([]int, len(cores))
		for i, core := range cores {
			pieces[i] = core.pieces
		}
		order := make([]int, len(cores))
		for i := range order {
			order[i] = i
		}
		for len(order) >= full {
			count := len(order) / full
			kept := []int{}
			for i := range count {
				picked := order[i*full : i*full+full]
				visit(picked)
				for _, j := range picked {
					if pieces[j] -= h.shareBase; pieces[j] > 0 {
						kept = append(kept, j)
					}
				}
			}
			order = append(kept, order[count*full:]...)
		}
		return
	}

	cpuHeap := make(cpuCoreHeap, len(cores))
	for i, core := range cores {
		cpuHeap[i] = &cpuCore{ID: core.ID, pieces: core.pieces, index: i}
	}
	heap.Init(&cpuHeap)
	picked := make([]int, full)
	resourcesToPush := make([]*cpuCore, 0, full)
	for cpuHeap.Len() >= full {
		resourcesToPush = resourcesToPush[:0]
		for n := range full {
			core := heap.Pop(&cpuHeap).(*cpuCore)
			picked[n] = core.index
			core.pieces -= h.shareBase
			if core.pieces > 0 {
				resourcesToPush = append(resourcesToPush, core)
			}
		}
		visit(picked)
		for _, core := range resourcesToPush {
			heap.Push(&cpuHeap, core)
		}
	}
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

func (h *host) countFragmentCPUPlans(cores []*cpuCore, fragment int) int {
	count := 0
	for _, core := range cores {
		count += core.pieces / fragment
	}
	return count
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

func CountCPUPlans(resourceInfo *types.NodeResourceInfo, originCPUMap types.CPUMap, shareBase, maxFragmentCores int, req *types.WorkloadResourceRequest) int {
	if len(resourceInfo.Capacity.NUMA) > 0 {
		return len(GetCPUPlans(resourceInfo, originCPUMap, shareBase, maxFragmentCores, req))
	}
	availableResource := resourceInfo.GetAvailableResource()
	return doCountCPUPlans(originCPUMap, availableResource.CPUMap, availableResource.Memory, shareBase, maxFragmentCores, req.CPURequest, req.MemRequest)
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

func doCountCPUPlans(originCPUMap, availableCPUMap types.CPUMap, availableMemory int64, shareBase, maxFragmentCores int, cpuRequest float64, memoryRequest int64) int {
	h := newHost(availableCPUMap, shareBase, maxFragmentCores)

	if len(originCPUMap) > 0 {
		originH := newHost(originCPUMap, shareBase, maxFragmentCores)
		reorderByAffinity(originH, h)
	}

	count := h.countCPUPlans(cpuRequest)
	if memoryRequest > 0 {
		return min(count, int(availableMemory/memoryRequest))
	}
	return count
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

func byLoad(a, b *cpuCore) int {
	return cmp.Or(cmp.Compare(a.pieces, b.pieces), cmp.Compare(a.ID, b.ID))
}
