package complexscheduler

import (
	"container/heap"
	"sort"

	"github.com/projecteru2/core/types"
)

type resourceInfo struct {
	id     string
	pieces int64
}

type resourceInfoHeap []resourceInfo

// Len .
func (r resourceInfoHeap) Len() int {
	return len(r)
}

// Less .
func (r resourceInfoHeap) Less(i, j int) bool {
	return r[i].pieces > r[j].pieces
}

// Swap .
func (r resourceInfoHeap) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// Push .
func (r *resourceInfoHeap) Push(x interface{}) {
	*r = append(*r, x.(resourceInfo))
}

// Pop .
func (r *resourceInfoHeap) Pop() interface{} {
	old := *r
	n := len(old)
	x := old[n-1]
	*r = old[:n-1]
	return x
}

type host struct {
	full     []resourceInfo
	fragment []resourceInfo
	share    int
}

func newHost(resourceMap types.ResourceMap, share int) *host {
	result := &host{
		share:    share,
		full:     []resourceInfo{},
		fragment: []resourceInfo{},
	}
	for id, pieces := range resourceMap {
		// 整数核不应该切分
		if pieces >= int64(share) && pieces%int64(share) == 0 {
			// 只给 share 份
			result.full = append(result.full, resourceInfo{id: id, pieces: pieces})
		} else if pieces > 0 {
			result.fragment = append(result.fragment, resourceInfo{id: id, pieces: pieces})
		}
	}
	// 确保优先分配更碎片的核
	sort.Slice(result.fragment, func(i, j int) bool { return result.fragment[i].pieces < result.fragment[j].pieces })
	// 确保优先分配负重更大的整数核
	sort.Slice(result.full, func(i, j int) bool { return result.full[i].pieces < result.full[j].pieces })

	return result
}

func (h *host) getComplexResult(full int, fragment int64, maxShare int) []types.ResourceMap {
	if maxShare == -1 {
		maxShare = len(h.full) - full // 减枝，M == N 的情况下预留至少一个 full 量的核数
	} else {
		maxShare -= len(h.fragment)
	}

	// 计算默认情况下能部署多少个
	fragmentResultBase := h.getFragmentResult(fragment, h.fragment)
	fullResultBase := h.getFullResult(full, h.full)
	fragmentResultCount := len(fragmentResultBase)
	fullResultCount := len(fullResultBase)

	baseLine := min(fragmentResultCount, fullResultCount)
	fragmentResult := fragmentResultBase
	fullResult := fullResultBase
	for i := 1; i < maxShare+1; i++ {
		fragmentResultBase = h.getFragmentResult(fragment, append(h.fragment, h.full[:i]...))
		fullResultBase = h.getFullResult(full, h.full[i:])
		fragmentResultCount = len(fragmentResultBase)
		fullResultCount = len(fullResultBase)

		canDeployNum := min(fragmentResultCount, fullResultCount)
		if canDeployNum > baseLine {
			baseLine = canDeployNum
			fragmentResult = fragmentResultBase
			fullResult = fullResultBase
		}
	}

	result := []types.ResourceMap{}
	for i := 0; i < baseLine; i++ {
		r := types.ResourceMap{}
		for id, pieces := range fullResult[i] {
			r[id] += pieces
		}
		for id, pieces := range fragmentResult[i] {
			r[id] = pieces
		}
		result = append(result, r)
	}

	return result
}

func (h *host) getFragmentResult(fragment int64, resources []resourceInfo) []types.ResourceMap {
	resourceMaps := h.getFragmentsResult(resources, fragment)
	result := make([]types.ResourceMap, len(resourceMaps))
	for i, resourceMap := range resourceMaps {
		result[i] = resourceMap[0]
	}

	// to pass tests due to new algorithm returns unsorted list
	resourceIdx := map[string]int{}
	for idx, resource := range resources {
		resourceIdx[resource.id] = idx
	}
	sort.Slice(result, func(i, j int) bool {
		return resourceIdx[result[i].GetResourceID()] < resourceIdx[result[j].GetResourceID()]
	})
	return result
}

func (h *host) getFragmentsResult(resources []resourceInfo, fragments ...int64) (resourceMap [][]types.ResourceMap) {
	if len(fragments) == 0 {
		return
	}

	// shouldn't change resources slice
	resourcesBak := make([]resourceInfo, len(resources))
	copy(resourcesBak, resources)
	defer func() {
		copy(resources, resourcesBak)
	}()

	nFragments := len(fragments)
	nResources := len(resources)
	sort.Slice(fragments, func(i, j int) bool { return fragments[i] > fragments[j] }) // fragments is descendant
	var totalRequired int64
	for _, fragment := range fragments {
		totalRequired += fragment
	}

	for i := 0; i < nResources; i++ {
		sort.Slice(resources, func(i, j int) bool { return resources[i].pieces < resources[j].pieces })

		count := resources[i].pieces / totalRequired

		// plan on the same resource
		plan := []types.ResourceMap{}
		for _, fragment := range fragments {
			plan = append(plan, types.ResourceMap{resources[i].id: fragment})
		}
		for j := int64(0); j < count; j++ {
			resourceMap = append(resourceMap, plan)
		}
		resources[i].pieces -= count * totalRequired

		// plan on different resources
		plan = []types.ResourceMap{}
		refugees := []int64{} // refugees record the fragments not able to scheduled on resource[i]
		for _, fragment := range fragments {
			if resources[i].pieces >= fragment {
				resources[i].pieces -= fragment
				plan = append(plan, types.ResourceMap{resources[i].id: fragment})
			} else {
				refugees = append(refugees, fragment)
			}
		}

		if len(refugees) == nFragments {
			// resources[i] runs out of capacity, calculate resources[i+1]
			continue
		}

		// looking for resource(s) capable of taking in refugees
		for j := i + 1; j < nResources; j++ {
			fragments := refugees
			refugees = []int64{}
			for _, fragment := range fragments {
				if resources[j].pieces >= fragment {
					resources[j].pieces -= fragment
					plan = append(plan, types.ResourceMap{resources[j].id: fragment})
				} else {
					refugees = append(refugees, fragment)
				}
			}
			if len(refugees) == 0 {
				resourceMap = append(resourceMap, plan)
				break
			}
		}
		if len(refugees) > 0 {
			// fail to complete this plan
			break
		}
	}

	return resourceMap
}

func (h *host) getFullResult(full int, resources []resourceInfo) []types.ResourceMap {
	result := []types.ResourceMap{}
	resourceHeap := &resourceInfoHeap{}
	indexMap := map[string]int{}
	for i, resource := range resources {
		indexMap[resource.id] = i
		resourceHeap.Push(resourceInfo{id: resource.id, pieces: resource.pieces})
	}
	heap.Init(resourceHeap)

	for resourceHeap.Len() >= full {
		plan := types.ResourceMap{}
		resourcesToPush := []resourceInfo{}

		for i := 0; i < full; i++ {
			resource := heap.Pop(resourceHeap).(resourceInfo)
			plan[resource.id] = int64(h.share)

			resource.pieces -= int64(h.share)
			if resource.pieces > 0 {
				resourcesToPush = append(resourcesToPush, resource)
			}
		}

		result = append(result, plan)
		for _, resource := range resourcesToPush {
			heap.Push(resourceHeap, resource)
		}
	}

	// Try to ensure the effectiveness of the previous priority
	sumOfIds := func(r types.ResourceMap) int {
		sum := 0
		for id := range r {
			sum += indexMap[id]
		}
		return sum
	}

	sort.Slice(result, func(i, j int) bool { return sumOfIds(result[i]) < sumOfIds(result[j]) })

	return result
}

func (h *host) distributeOneRation(ration float64, maxShare int) []types.ResourceMap {
	ration *= float64(h.share)
	fullRequire := int64(ration) / int64(h.share)
	fragmentRequire := int64(ration) % int64(h.share)

	if fullRequire == 0 {
		if maxShare == -1 {
			// 这个时候就把所有的资源都当成碎片
			maxShare = len(h.full) + len(h.fragment)
		}
		diff := maxShare - len(h.fragment)
		h.fragment = append(h.fragment, h.full[:diff]...)

		return h.getFragmentResult(fragmentRequire, h.fragment)
	}

	if fragmentRequire == 0 {
		return h.getFullResult(int(fullRequire), h.full)
	}

	return h.getComplexResult(int(fullRequire), fragmentRequire, maxShare)
}

func (h *host) distributeMultipleRations(rations []int64) [][]types.ResourceMap {
	return h.getFragmentsResult(h.fragment, rations...)
}
