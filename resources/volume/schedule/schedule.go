package schedule

import (
	"container/heap"
	"math"
	"sort"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/volume/types"
)

type volume struct {
	device string
	size   int64
}

func (v *volume) LessThan(v1 *volume) bool {
	if v.size == v1.size {
		return v.device < v1.device
	}
	return v.size < v1.size
}

type volumes []*volume

// DeepCopy .
func (v volumes) DeepCopy() volumes {
	res := volumes{}
	for _, item := range v {
		res = append(res, &volume{device: item.device, size: item.size})
	}
	return res
}

type volumeHeap volumes

// Len .
func (v volumeHeap) Len() int {
	return len(v)
}

// Less .
func (v volumeHeap) Less(i, j int) bool {
	return v[i].LessThan(v[j])
}

// Swap .
func (v volumeHeap) Swap(i, j int) {
	v[i], v[j] = v[j], v[i]
}

// Push .
func (v *volumeHeap) Push(x interface{}) {
	*v = append(*v, x.(*volume))
}

// Pop .
func (v *volumeHeap) Pop() interface{} {
	old := *v
	n := len(old)
	x := old[n-1]
	*v = old[:n-1]
	return x
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type host struct {
	maxDeployCount int
	usedVolumes    volumes
	unusedVolumes  volumes
}

func newHost(resourceInfo *types.NodeResourceInfo, maxDeployCount int) *host {
	h := &host{
		maxDeployCount: maxDeployCount,
		usedVolumes:    []*volume{},
		unusedVolumes:  []*volume{},
	}

	for device, size := range resourceInfo.Capacity.Volumes {
		used := resourceInfo.Usage.Volumes[device]
		if used == 0 {
			h.unusedVolumes = append(h.unusedVolumes, &volume{device: device, size: size})
		} else {
			h.usedVolumes = append(h.usedVolumes, &volume{device: device, size: size - used})
		}
	}

	sort.SliceStable(h.unusedVolumes, func(i, j int) bool { return h.unusedVolumes[i].LessThan(h.unusedVolumes[j]) })
	sort.SliceStable(h.usedVolumes, func(i, j int) bool { return h.usedVolumes[i].LessThan(h.usedVolumes[j]) })
	return h
}

func (h *host) getMonoPlan(monoRequests types.VolumeBindings, totalRequestSize int64, volume *volume) types.VolumePlan {
	if volume.size < totalRequestSize {
		return nil
	}
	volumePlan := types.VolumePlan{}

	volumeSize := volume.size
	for _, req := range monoRequests {
		size := int64(float64(req.SizeInBytes) / float64(totalRequestSize) * float64(volumeSize))
		volumePlan[req] = types.VolumeMap{volume.device: size}
		volume.size -= size
	}

	if volume.size != 0 {
		for _, volumeMap := range volumePlan {
			volumeMap[volumeMap.GetDevice()] += volume.size
			break
		}
	}

	return volumePlan
}

func (h *host) getMonoPlans(monoRequests types.VolumeBindings) ([]types.VolumePlan, int) {
	if len(monoRequests) == 0 {
		return []types.VolumePlan{}, h.maxDeployCount
	}
	if len(h.unusedVolumes) == 0 {
		return []types.VolumePlan{}, 0
	}

	totalSize := int64(0)
	for _, req := range monoRequests {
		totalSize += req.SizeInBytes
	}

	volumes := h.unusedVolumes.DeepCopy()
	volumePlans := []types.VolumePlan{}

	// h.unusedVolumes have already been sorted
	for _, volume := range volumes {
		if volumePlan := h.getMonoPlan(monoRequests, totalSize, volume); volumePlan != nil {
			volumePlans = append(volumePlans, volumePlan)
		}
	}

	return volumePlans, len(volumePlans)
}

func (h *host) getNormalPlan(vHeap *volumeHeap, normalRequests types.VolumeBindings) types.VolumePlan {
	volumePlan := types.VolumePlan{}
	for reqIndex := 0; reqIndex < len(normalRequests); reqIndex++ {
		req := normalRequests[reqIndex]
		volumeToPush := []*volume{}
		allocated := false

		for vHeap.Len() > 0 {
			volume := heap.Pop(vHeap).(*volume)
			if volume.size >= req.SizeInBytes {
				volumePlan[req] = types.VolumeMap{volume.device: req.SizeInBytes}
				allocated = true
				volume.size -= req.SizeInBytes
				if volume.size > 0 {
					volumeToPush = append(volumeToPush, volume)
				}
				break
			}
		}

		for _, volume := range volumeToPush {
			heap.Push(vHeap, volume)
		}

		if !allocated {
			return nil
		}
	}

	return volumePlan
}

func (h *host) getNormalPlans(normalRequests types.VolumeBindings) ([]types.VolumePlan, int) {
	if len(normalRequests) == 0 {
		return []types.VolumePlan{}, h.maxDeployCount
	}

	vh := volumeHeap(h.usedVolumes.DeepCopy())
	vHeap := &vh
	heap.Init(vHeap)

	volumePlans := []types.VolumePlan{}

	for len(volumePlans) <= h.maxDeployCount {
		if volumePlan := h.getNormalPlan(vHeap, normalRequests); volumePlan != nil {
			volumePlans = append(volumePlans, volumePlan)
		} else {
			break
		}
	}
	return volumePlans, len(volumePlans)
}

func (h *host) getUnlimitedPlans(normalPlans, monoPlans []types.VolumePlan, unlimitedRequests types.VolumeBindings) []types.VolumePlan {
	capacity := len(normalPlans)

	volumes := append(h.usedVolumes.DeepCopy(), h.unusedVolumes.DeepCopy()...)
	volumeMap := map[string]*volume{}
	for _, volume := range volumes {
		volumeMap[volume.device] = volume
	}

	// apply normal changes
	for _, plan := range normalPlans {
		for _, vm := range plan {
			for device, size := range vm {
				volumeMap[device].size -= size
			}
		}
	}

	// apply mono changes
	for _, plan := range monoPlans {
		for _, vm := range plan {
			for device, size := range vm {
				volumeMap[device].size -= size
			}
		}
	}

	// select the volume with the largest size
	maxVolume := volumes[0]
	for i := range volumes {
		if volumes[i].size > maxVolume.size {
			maxVolume = volumes[i]
		}
	}

	plans := []types.VolumePlan{}
	for i := 0; i < capacity; i++ {
		plan := types.VolumePlan{}
		for _, req := range unlimitedRequests {
			plan[req] = types.VolumeMap{maxVolume.device: req.SizeInBytes}
		}
		plans = append(plans, plan)
	}

	return plans
}

func (h *host) getVolumePlans(volumeBindings types.VolumeBindings) []types.VolumePlan {
	if len(h.unusedVolumes) == 0 && len(h.usedVolumes) == 0 {
		return nil
	}

	normalRequests, monoRequests, unlimitedRequests := h.classifyVolumeBindings(volumeBindings)

	minNormalRequestSize := int64(math.MaxInt)
	for _, normalRequest := range normalRequests {
		if normalRequest.SizeInBytes < minNormalRequestSize {
			minNormalRequestSize = normalRequest.SizeInBytes
		}
	}

	// get baseline
	normalPlans, normalCapacity := h.getNormalPlans(normalRequests)
	monoPlans, monoCapacity := h.getMonoPlans(monoRequests)
	bestCapacity := min(monoCapacity, normalCapacity)
	bestVolumePlans := [2][]types.VolumePlan{normalPlans[:min(bestCapacity, len(normalPlans))], monoPlans[:min(bestCapacity, len(monoPlans))]}

	for monoCapacity > normalCapacity {
		// convert an unused volume to used volume
		p := sort.Search(len(h.unusedVolumes), func(i int) bool { return h.unusedVolumes[i].size >= minNormalRequestSize })
		// if no volume meets the requirement, just stop
		if p == len(h.unusedVolumes) {
			break
		}
		v := h.unusedVolumes[p]
		h.unusedVolumes = append(h.unusedVolumes[:p], h.unusedVolumes[p+1:]...)
		h.usedVolumes = append(h.usedVolumes, v)

		normalPlans, normalCapacity = h.getNormalPlans(normalRequests)
		monoPlans, monoCapacity = h.getMonoPlans(monoRequests)
		capacity := min(monoCapacity, normalCapacity)
		if capacity > bestCapacity {
			bestCapacity = capacity
			bestVolumePlans = [2][]types.VolumePlan{normalPlans[:min(len(normalPlans), capacity)], monoPlans[:min(len(monoPlans), capacity)]}
		}
	}

	normalPlans, monoPlans = bestVolumePlans[0], bestVolumePlans[1]
	unlimitedPlans := h.getUnlimitedPlans(normalPlans, monoPlans, unlimitedRequests)

	resVolumePlans := []types.VolumePlan{}
	merge := func(plan types.VolumePlan, plans []types.VolumePlan, index int) {
		if index < len(plans) {
			plan.Merge(plans[index])
		}
	}

	for i := 0; i < bestCapacity; i++ {
		plan := types.VolumePlan{}
		merge(plan, normalPlans, i)
		merge(plan, monoPlans, i)
		merge(plan, unlimitedPlans, i)
		resVolumePlans = append(resVolumePlans, plan)
	}
	return resVolumePlans
}

func (h *host) classifyVolumeBindings(volumeBindings types.VolumeBindings) (normalRequests, monoRequests, unlimitedRequests types.VolumeBindings) {
	for _, binding := range volumeBindings {
		switch {
		case binding.RequireScheduleMonopoly():
			monoRequests = append(monoRequests, binding)
		case binding.RequireScheduleUnlimitedQuota():
			unlimitedRequests = append(unlimitedRequests, binding)
		case binding.RequireSchedule():
			normalRequests = append(normalRequests, binding)
		}
	}

	sort.SliceStable(monoRequests, func(i, j int) bool { return monoRequests[i].SizeInBytes < monoRequests[j].SizeInBytes })
	sort.SliceStable(normalRequests, func(i, j int) bool { return normalRequests[i].SizeInBytes < normalRequests[j].SizeInBytes })

	return normalRequests, monoRequests, unlimitedRequests
}

func (h *host) classifyAffinityRequests(requests types.VolumeBindings, existing types.VolumePlan) (affinity map[*types.VolumeBinding]types.VolumeMap, nonAffinity map[*types.VolumeBinding]types.VolumeMap) {
	affinity = map[*types.VolumeBinding]types.VolumeMap{}
	nonAffinity = map[*types.VolumeBinding]types.VolumeMap{}

	for _, req := range requests {
		found := false
		for binding, volumeMap := range existing {
			if req.Source == binding.Source && req.Destination == binding.Destination && req.Flags == binding.Flags {
				affinity[req] = volumeMap
				found = true
				break
			}
		}
		if !found {
			nonAffinity[req] = nil
		}
	}
	return affinity, nonAffinity
}

func (h *host) getVolumeByDevice(device string) *volume {
	for _, volume := range h.usedVolumes {
		if volume.device == device {
			return volume
		}
	}
	for _, volume := range h.unusedVolumes {
		if volume.device == device {
			return volume
		}
	}
	return nil
}

func (h *host) getAffinityPlan(volumeRequest types.VolumeBindings, existing types.VolumePlan) types.VolumePlan {
	normalRequests, monoRequests, unlimitedRequests := h.classifyVolumeBindings(volumeRequest)
	needRescheduleRequests := types.VolumeBindings{}
	volumePlan := types.VolumePlan{}
	for binding, volumeMap := range existing {
		if !binding.RequireScheduleMonopoly() {
			volumePlan[binding] = volumeMap
		}
	}

	commonProcess := func(requests types.VolumeBindings) error {
		affinity, nonAffinity := h.classifyAffinityRequests(requests, existing)
		for binding, volumeMap := range affinity {
			device := volumeMap.GetDevice()
			size := volumeMap.GetSize()
			// check if the device has enough space
			diff := binding.SizeInBytes - size
			if diff > h.getVolumeByDevice(device).size {
				logrus.Errorf("[getAffinityPlan] no space to expand, %v remains %v, requires %v", device, h.getVolumeByDevice(device).size, diff)
				return types.ErrInsufficientResource
			}
			volumePlan.Merge(types.VolumePlan{binding: types.VolumeMap{device: binding.SizeInBytes}})
		}
		for binding := range nonAffinity {
			needRescheduleRequests = append(needRescheduleRequests, binding)
		}
		return nil
	}

	// normal
	if err := commonProcess(normalRequests); err != nil {
		return nil
	}

	// mono
	totalRequestSize := int64(0)
	totalVolumeSize := int64(0)
	bindingMap := map[[3]string]*types.VolumeBinding{}
	for binding, volumeMap := range existing {
		if binding.RequireScheduleMonopoly() {
			totalRequestSize += binding.SizeInBytes
			totalVolumeSize += volumeMap.GetSize()
			bindingMap[binding.GetMapKey()] = binding
		}
	}

	// update bindings
	for _, binding := range monoRequests {
		totalRequestSize += binding.SizeInBytes
		key := binding.GetMapKey()
		if vb, ok := bindingMap[key]; ok {
			vb.SizeInBytes += binding.SizeInBytes
		} else {
			bindingMap[key] = binding
		}
	}
	requestBindings := types.VolumeBindings{}
	for _, binding := range bindingMap {
		requestBindings = append(requestBindings, binding)
	}

	affinity, nonAffinity := h.classifyAffinityRequests(requestBindings, existing)
	// if there is no affinity plan: all reschedule
	if len(affinity) == 0 {
		for binding := range nonAffinity {
			needRescheduleRequests = append(needRescheduleRequests, binding)
		}
	} else {
		// if there is any affinity plan: don't reschedule
		// use the first volume map to get the whole mono volume plan
		for _, volumeMap := range affinity {
			for _, volume := range h.usedVolumes {
				if volume.device == volumeMap.GetDevice() {
					volume.size += totalVolumeSize
					volumePlan.Merge(h.getMonoPlan(requestBindings, totalRequestSize, volume))
					break
				}
			}
			break
		}
	}

	// unlimited
	if err := commonProcess(unlimitedRequests); err != nil {
		return nil
	}

	if len(needRescheduleRequests) == 0 {
		return volumePlan
	}

	volumePlans := h.getVolumePlans(needRescheduleRequests)
	if len(volumePlans) == 0 {
		return nil
	}
	volumePlan.Merge(volumePlans[0])
	return volumePlan
}

// GetAffinityPlan .
func GetAffinityPlan(resourceInfo *types.NodeResourceInfo, volumeRequest types.VolumeBindings, existing types.VolumePlan) types.VolumePlan {
	h := newHost(resourceInfo, 1)
	return h.getAffinityPlan(volumeRequest, existing)
}

// GetVolumePlans .
func GetVolumePlans(resourceInfo *types.NodeResourceInfo, volumeRequest types.VolumeBindings, maxDeployCount int) []types.VolumePlan {
	h := newHost(resourceInfo, maxDeployCount)
	return h.getVolumePlans(volumeRequest)
}
