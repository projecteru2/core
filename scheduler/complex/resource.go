package complexscheduler

import (
	"sort"

	"github.com/projecteru2/core/types"
)

type resourceInfo struct {
	id     string
	pieces int
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
		if pieces >= share && pieces%share == 0 {
			// 只给 share 份
			result.full = append(result.full, resourceInfo{id: id, pieces: pieces})
		} else {
			result.fragment = append(result.fragment, resourceInfo{id: id, pieces: pieces})
		}
	}
	// 确保优先分配更碎片的核
	sort.Slice(result.fragment, func(i, j int) bool { return result.fragment[i].pieces < result.fragment[j].pieces })
	// 确保优先分配负重更大的整数核
	sort.Slice(result.full, func(i, j int) bool { return result.full[i].pieces < result.full[j].pieces })

	return result
}

func (h *host) getComplexResult(full, fragment, maxShareCore int) []types.ResourceMap {
	if maxShareCore == -1 {
		maxShareCore = len(h.full) - full // 减枝，M == N 的情况下预留至少一个 full 量的核数
	} else {
		maxShareCore -= len(h.fragment)
	}

	// 计算默认情况下能部署多少个
	fragmentResultBase := h.getFragmentResult(fragment, h.fragment)
	fullResultBase := h.getFullResult(full, h.full)
	fragmentResultCount := len(fragmentResultBase)
	fullResultCount := len(fullResultBase)

	baseLine := min(fragmentResultCount, fullResultCount)
	fragmentResult := fragmentResultBase
	fullResult := fullResultBase
	for i := 1; i < maxShareCore+1; i++ {
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
		for no, pieces := range fullResult[i] {
			if _, ok := r[no]; ok {
				r[no] += pieces
			} else {
				r[no] = pieces
			}
		}
		for no, pieces := range fragmentResult[i] {
			r[no] = pieces
		}
		result = append(result, r)
	}

	return result
}

func (h *host) getFragmentResult(fragment int, resources []resourceInfo) (result []types.ResourceMap) {
	resourceMaps := h.getFragmentsResult(resources, fragment)
	for i, ResourceMap := range resourceMaps {
		result[i] = ResourceMap[0]
	}
	return result
}

func (h *host) getFragmentsResult(resources []resourceInfo, fragments ...int) (resourceMap [][]types.ResourceMap) {
	// shouldn't change resources slice
	defer func(resourceBak []resourceInfo) {
		resources = resourceBak
	}(resources)

	nResources := len(resources)
	sort.Sort(sort.Reverse(sort.IntSlice(fragments))) // fragments is descendant
	totalRequired := 0
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
		for j := 0; j < count; j++ {
			resourceMap = append(resourceMap, plan)
		}

		// plan on different resources
		plan = []types.ResourceMap{}
		remainder := resources[i].pieces - count*totalRequired
		refugees := []int{} // refugees record the fragments not able to scheduled on resource[i]
		for _, fragment := range fragments {
			if remainder > fragment {
				remainder -= fragment
				plan = append(plan, types.ResourceMap{resources[i].id: fragment})
			} else {
				refugees = append(refugees, fragment)
			}
		}
		// looking for resource(s) capable of taking in refugees
		for j := i + 1; j < nResources; j++ {
			fragments := refugees
			refugees = []int{}
			for _, fragment := range fragments {
				if resources[j].pieces > fragment {
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

	return
}

func (h *host) getFullResult(full int, resources []resourceInfo) []types.ResourceMap {
	result := []types.ResourceMap{}
	count := len(resources) / full
	newResources := []resourceInfo{}
	for i := 0; i < count; i++ {
		plan := types.ResourceMap{}
		for j := i * full; j < i*full+full; j++ {
			// 洗掉没配额的
			last := resources[j].pieces - h.share
			if last > 0 {
				newResources = append(newResources, resourceInfo{resources[j].id, last})
			}
			plan[resources[j].id] = h.share
		}
		result = append(result, plan)
	}

	if len(newResources)/full > 0 {
		return append(result, h.getFullResult(full, newResources)...)
	}
	return result
}

func (h *host) distributeOneRation(ration float64, maxShare int) []types.ResourceMap {
	ration = ration * float64(h.share)
	fullRequire := int(ration) / h.share
	fragmentRequire := int(ration) % h.share

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
		return h.getFullResult(fullRequire, h.full)
	}

	return h.getComplexResult(fullRequire, fragmentRequire, maxShare)
}

func (h *host) distributeMultipleRations(rations []int) [][]types.ResourceMap {
	return h.getFragmentsResult(h.fragment, rations...)
}
