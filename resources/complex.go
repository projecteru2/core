package resources

import (
	"container/heap"
	"sort"
)

// ResourceMap {["0"]10000, ["1"]10000}
type ResourceMap map[string]int64

// GetResourceID returns device name such as "/sda0"
// GetResourceID only works for VolumeMap with single key
func (c ResourceMap) GetResourceID() (key string) {
	for k := range c {
		key = k
		break
	}
	return
}

// GetRation returns scheduled size from device
// GetRation only works for VolumeMap with single key
func (c ResourceMap) GetRation() int64 {
	return c[c.GetResourceID()]
}

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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type host struct {
	full     []resourceInfo
	fragment []resourceInfo
	share    int
}

// NewHost creates a new host
func NewHost(resourceMap ResourceMap, share int) *host {
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

func (h *host) getComplexResult(full int, fragment int64, maxShare int) []ResourceMap {
	if maxShare == -1 {
		maxShare = len(h.full) - full // 减枝，M == N 的情况下预留至少一个 full 量的核数
	} else {
		maxShare -= len(h.fragment) // 最大碎片核数 - 碎片核数，这里maxShare相当于一个diff
	}

	// 计算默认情况下能部署多少个
	fragmentResultBase := h.getFragmentResult(fragment, h.fragment)
	fullResultBase := h.getFullResult(full, h.full)
	fragmentResultCount := len(fragmentResultBase)
	fullResultCount := len(fullResultBase)

	// 先计算整数核都是整数核的情况，整数核部分和小数核部分各能部署几个
	baseLine := min(fragmentResultCount, fullResultCount)
	fragmentResult := fragmentResultBase
	fullResult := fullResultBase
	// 依次把整数核分配给小数核，看情况能不能有所好转
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

	result := []ResourceMap{}
	for i := 0; i < baseLine; i++ {
		r := ResourceMap{}
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

func (h *host) getFragmentResult(fragment int64, resources []resourceInfo) []ResourceMap {
	resourceMaps := h.getFragmentsResult(resources, fragment)
	result := make([]ResourceMap, len(resourceMaps))
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

func (h *host) getFragmentsResult(resources []resourceInfo, fragments ...int64) (resourceMap [][]ResourceMap) {
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
	// fragment降序排列
	sort.Slice(fragments, func(i, j int) bool { return fragments[i] > fragments[j] }) // fragments is descendant
	var totalRequired int64
	for _, fragment := range fragments {
		totalRequired += fragment
	}

	for i := 0; i < nResources; i++ {
		// resources 升序排列
		sort.Slice(resources, func(i, j int) bool { return resources[i].pieces < resources[j].pieces })

		count := resources[i].pieces / totalRequired

		// plan on the same resource
		plan := []ResourceMap{}
		for _, fragment := range fragments {
			plan = append(plan, ResourceMap{resources[i].id: fragment})
		}
		for j := int64(0); j < count; j++ {
			resourceMap = append(resourceMap, plan)
		}
		resources[i].pieces -= count * totalRequired

		// plan on different resources
		plan = []ResourceMap{}
		refugees := []int64{} // refugees record the fragments not able to scheduled on resource[i]
		for _, fragment := range fragments {
			if resources[i].pieces >= fragment {
				resources[i].pieces -= fragment
				plan = append(plan, ResourceMap{resources[i].id: fragment})
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
					plan = append(plan, ResourceMap{resources[j].id: fragment})
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

func (h *host) getFullResult(full int, resources []resourceInfo) []ResourceMap {
	result := []ResourceMap{}
	resourceHeap := &resourceInfoHeap{}
	indexMap := map[string]int{}
	for i, resource := range resources {
		indexMap[resource.id] = i
		resourceHeap.Push(resourceInfo{id: resource.id, pieces: resource.pieces})
	}
	heap.Init(resourceHeap)

	for resourceHeap.Len() >= full {
		plan := ResourceMap{}
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
	sumOfIds := func(r ResourceMap) int {
		sum := 0
		for id := range r {
			sum += indexMap[id]
		}
		return sum
	}

	sort.Slice(result, func(i, j int) bool { return sumOfIds(result[i]) < sumOfIds(result[j]) })

	return result
}

// DistributeOneRation .
func (h *host) DistributeOneRation(ration float64, maxShare int) []ResourceMap {
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

// DistributeMultipleRations .
func (h *host) DistributeMultipleRations(rations []int64) [][]ResourceMap {
	return h.getFragmentsResult(h.fragment, rations...)
}
