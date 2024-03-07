package schedule

import (
	"container/heap"
	"sort"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
	"github.com/projecteru2/core/utils"
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

// Len .
func (c cpuCoreHeap) Len() int {
	return len(c)
}

// Less .
func (c cpuCoreHeap) Less(i, j int) bool {
	return !c[i].Less(c[j])
}

// Swap .
func (c cpuCoreHeap) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

// Push .
func (c *cpuCoreHeap) Push(x any) {
	*c = append(*c, x.(*cpuCore))
}

// Pop .
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

// GetCPUPlans .
func GetCPUPlans(resourceInfo *types.NodeResourceInfo, originCPUMap types.CPUMap, shareBase int, maxFragmentCores int, req *types.WorkloadResourceRequest) []*types.CPUPlan {
	cpuPlans := []*types.CPUPlan{}
	availableResource := resourceInfo.GetAvailableResource()

	numaCPUMap := map[string]types.CPUMap{}
	for cpuID, numaNodeID := range resourceInfo.Capacity.NUMA {
		if _, ok := numaCPUMap[numaNodeID]; !ok {
			numaCPUMap[numaNodeID] = types.CPUMap{}
		}
		numaCPUMap[numaNodeID][cpuID] = availableResource.CPUMap[cpuID]
	}

	// get cpu plan for each numa node
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

	// get cpu plan with the remaining resource
	crossNUMACPUPlans := doGetCPUPlans(originCPUMap, availableResource.CPUMap, availableResource.Memory, shareBase, maxFragmentCores, req.CPURequest, req.MemRequest)
	for _, workloadCPUMap := range crossNUMACPUPlans {
		cpuPlans = append(cpuPlans, &types.CPUPlan{
			CPUMap: workloadCPUMap,
		})
	}

	return cpuPlans
}

// ensure that the old cpu core will still be allocated first
func reorderByAffinity(oldH, newH *host) {
	oldFull := map[string]int{}
	oldFragment := map[string]int{}

	for i, core := range oldH.fullCores {
		oldFull[core.ID] = i + 1
	}
	for i, core := range oldH.fragmentCores {
		oldFragment[core.ID] = i + 1
	}

	sortFunc := func(orderMap map[string]int, cores []*cpuCore) func(i, j int) bool {
		return func(i, j int) bool {
			idxI := orderMap[cores[i].ID]
			idxJ := orderMap[cores[j].ID]

			if idxI == 0 && idxJ == 0 {
				return i < j
			}
			if idxI == 0 || idxJ == 0 {
				return idxI > idxJ
			}
			return idxI < idxJ
		}
	}

	sort.SliceStable(newH.fullCores, sortFunc(oldFull, newH.fullCores))
	sort.SliceStable(newH.fragmentCores, sortFunc(oldFragment, newH.fragmentCores))
	newH.affinity = true
}

// doGetCPUPlans .
func doGetCPUPlans(originCPUMap, availableCPUMap types.CPUMap, availableMemory int64, shareBase int, maxFragmentCores int, cpuRequest float64, memoryRequest int64) []types.CPUMap {
	h := newHost(availableCPUMap, shareBase, maxFragmentCores)

	// affinity
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

func newHost(cpuMap types.CPUMap, shareBase int, maxFragmentCores int) *host {
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

	sortFunc := func(cores []*cpuCore) func(i, j int) bool {
		return func(i, j int) bool {
			// give priority to the CPU cores with higher load
			return cores[i].Less(cores[j])
		}
	}

	sort.SliceStable(h.fullCores, sortFunc(h.fullCores))
	sort.SliceStable(h.fragmentCores, sortFunc(h.fragmentCores))

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
		diff := h.maxFragmentCores - len(h.fragmentCores)
		h.fragmentCores = append(h.fragmentCores, h.fullCores[:diff]...)
		h.fullCores = h.fullCores[diff:]
		return h.getFragmentCPUPlans(h.fragmentCores, fragment)
	}

	fragmentCapacityMap := map[string]int{}
	totalFragmentCapacity := 0 // for lazy loading
	bestCPUPlans := [2][]types.CPUMap{h.getFullCPUPlans(h.fullCores, full), h.getFragmentCPUPlans(h.fragmentCores, fragment)}
	bestCapacity := utils.Min(len(bestCPUPlans[0]), len(bestCPUPlans[1]))

	for _, core := range h.fullCores {
		fragmentCapacityMap[core.ID] = core.pieces / fragment
	}

	for _, core := range h.fragmentCores {
		fragmentCapacityMap[core.ID] = core.pieces / fragment
		totalFragmentCapacity += fragmentCapacityMap[core.ID]
	}

	for len(h.fragmentCores) < h.maxFragmentCores {
		// convert a full core to fragment core
		newFragmentCore := h.fullCores[0]
		h.fragmentCores = append(h.fragmentCores, newFragmentCore)
		h.fullCores = h.fullCores[1:]
		totalFragmentCapacity += fragmentCapacityMap[newFragmentCore.ID]

		fullCPUPlans := h.getFullCPUPlans(h.fullCores, full)
		capacity := utils.Min(len(fullCPUPlans), totalFragmentCapacity)
		if capacity > bestCapacity {
			bestCPUPlans[0] = fullCPUPlans
			bestCPUPlans[1] = h.getFragmentCPUPlans(h.fragmentCores, fragment)
			bestCapacity = capacity
		}
	}

	cpuPlans := []types.CPUMap{}
	for i := 0; i < bestCapacity; i++ {
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

		for i := 0; i < full; i++ {
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

	// Try to ensure the effectiveness of the previous priority
	sumOfIDs := func(c types.CPUMap) int {
		sum := 0
		for ID := range c {
			sum += indexMap[ID]
		}
		return sum
	}

	sort.Slice(result, func(i, j int) bool { return sumOfIDs(result[i]) < sumOfIDs(result[j]) })

	return result
}

func (h *host) getFullCPUPlansWithAffinity(cores []*cpuCore, full int) []types.CPUMap {
	result := []types.CPUMap{}

	for len(cores) >= full {
		count := len(cores) / full
		tempCores := []*cpuCore{}
		for i := 0; i < count; i++ {
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
		for i := 0; i < core.pieces/fragment; i++ {
			result = append(result, types.CPUMap{core.ID: fragment})
		}
	}
	return result
}
