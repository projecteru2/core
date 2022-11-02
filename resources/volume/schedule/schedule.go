package schedule

import (
	"container/heap"
	"math"
	"sort"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
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

type host struct {
	maxDeployCount int
	usedVolumes    volumes
	unusedVolumes  volumes
	disks          types.Disks
}

func newHost(resourceInfo *types.NodeResourceInfo, maxDeployCount int) *host {
	h := &host{
		maxDeployCount: maxDeployCount,
		usedVolumes:    []*volume{},
		unusedVolumes:  []*volume{},
		disks:          resourceInfo.Capacity.Disks.DeepCopy(),
	}

	h.disks.Sub(resourceInfo.Usage.Disks)

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

// getDiskByPath finds the disk by given path. will return an empty disk if not found.
func (h *host) getDiskByPath(path string) *types.Disk {
	if disk := h.disks.GetDiskByPath(path); disk != nil {
		return disk
	}
	return &types.Disk{ReadIOPS: 0, WriteIOPS: 0, ReadBPS: 0, WriteBPS: 0}
}

// isDiskIOPSQuotaQualified returns true if the disk is qualified for the host.
func (h *host) isDiskIOPSQuotaQualified(disk *types.Disk, req *types.VolumeBinding) bool {
	return disk.ReadBPS >= req.ReadBPS && disk.WriteBPS >= req.WriteBPS && disk.ReadIOPS >= req.ReadIOPS && disk.WriteIOPS >= req.WriteIOPS
}

// decreaseIOPSQuota deducts the quota from the disk.
func (h *host) decreaseIOPSQuota(disk *types.Disk, req *types.VolumeBinding) {
	disk.ReadIOPS -= req.ReadIOPS
	disk.WriteIOPS -= req.WriteIOPS
	disk.ReadBPS -= req.ReadBPS
	disk.WriteBPS -= req.WriteBPS
}

// increaseIOPSQuota increases the quota from the disk.
func (h *host) increaseIOPSQuota(disk *types.Disk, req *types.VolumeBinding) {
	disk.ReadIOPS += req.ReadIOPS
	disk.WriteIOPS += req.WriteIOPS
	disk.ReadBPS += req.ReadBPS
	disk.WriteBPS += req.WriteBPS
}

// getMonoPlan try to allocate the requests on the given volume
func (h *host) getMonoPlan(monoRequests types.VolumeBindings, volume *volume) (types.VolumePlan, *types.Disk, error) {
	totalSize := utils.Sum(utils.Map(monoRequests, func(req *types.VolumeBinding) int64 { return req.SizeInBytes }))
	totalReadIOPS := utils.Sum(utils.Map(monoRequests, func(req *types.VolumeBinding) int64 { return req.ReadIOPS }))
	totalWriteIOPS := utils.Sum(utils.Map(monoRequests, func(req *types.VolumeBinding) int64 { return req.WriteIOPS }))
	totalReadBPS := utils.Sum(utils.Map(monoRequests, func(req *types.VolumeBinding) int64 { return req.ReadBPS }))
	totalWriteBPS := utils.Sum(utils.Map(monoRequests, func(req *types.VolumeBinding) int64 { return req.WriteBPS }))

	// check if volume size is enough
	if volume.size < totalSize {
		return nil, nil, coretypes.ErrInsufficientResource
	}

	// check if disk IOPS quota is enough
	disk := h.getDiskByPath(volume.device)
	if !h.isDiskIOPSQuotaQualified(disk, &types.VolumeBinding{SizeInBytes: totalSize, ReadIOPS: totalReadIOPS, WriteIOPS: totalWriteIOPS, ReadBPS: totalReadBPS, WriteBPS: totalWriteBPS}) {
		return nil, nil, coretypes.ErrInsufficientResource
	}

	volumePlan := types.VolumePlan{}
	volumeSize := volume.size

	// allocate proportionally
	for _, req := range monoRequests {
		size := int64(float64(req.SizeInBytes) / float64(totalSize) * float64(volumeSize))
		volumePlan[req] = types.VolumeMap{volume.device: size}
		volume.size -= size
	}

	// allocate the remaining quota to the first request
	if volume.size != 0 {
		for _, volumeMap := range volumePlan {
			volumeMap[volumeMap.GetDevice()] += volume.size
			volume.size = 0
			break
		}
	}

	diskPlan := &types.Disk{
		Device:    disk.Device,
		Mounts:    disk.Mounts,
		ReadIOPS:  totalReadIOPS,
		WriteIOPS: totalWriteIOPS,
		ReadBPS:   totalReadBPS,
		WriteBPS:  totalWriteBPS,
	}
	// decrease IOPS quota
	h.disks.Sub(types.Disks{diskPlan})

	return volumePlan, diskPlan, nil
}

func (h *host) getMonoPlans(monoRequests types.VolumeBindings) ([]types.VolumePlan, []types.Disks) {
	if len(monoRequests) == 0 {
		return utils.GenerateSlice(h.maxDeployCount, func() types.VolumePlan {
				return types.VolumePlan{}
			}), utils.GenerateSlice(h.maxDeployCount, func() types.Disks {
				return types.Disks{}
			})
	}
	if len(h.unusedVolumes) == 0 {
		return nil, nil
	}

	volumePlans := []types.VolumePlan{}
	diskPlans := []types.Disks{}

	// h.unusedVolumes is sorted by size, so we can allocate the volumes in order
	for _, volume := range h.unusedVolumes {
		volumePlan, diskPlan, err := h.getMonoPlan(monoRequests, volume)
		if err != nil {
			continue
		}
		volumePlans = append(volumePlans, volumePlan)
		diskPlans = append(diskPlans, types.Disks{diskPlan})
	}

	return volumePlans, diskPlans
}

func (h *host) getNormalPlan(normalRequests types.VolumeBindings) (types.VolumePlan, types.Disks, error) {
	volumeHeap := volumeHeap(h.usedVolumes)
	heap.Init(&volumeHeap)

	volumePlan := types.VolumePlan{}
	diskPlan := types.Disks{}

	if len(normalRequests) == 0 {
		return volumePlan, diskPlan, nil
	}

	// normalRequests is sorted by size, so we can allocate the volumes in order
	for _, req := range normalRequests {
		volumeToPush := []*volume{}
		allocated := false

		for volumeHeap.Len() > 0 {
			volume := heap.Pop(&volumeHeap).(*volume)
			disk := h.getDiskByPath(volume.device)
			// check if the size is enough
			if volume.size < req.SizeInBytes || !h.isDiskIOPSQuotaQualified(disk, req) {
				volumeToPush = append(volumeToPush, volume)
				continue
			}
			// decrease resource and generate plans
			h.decreaseIOPSQuota(disk, req)
			volume.size -= req.SizeInBytes
			volumePlan[req] = types.VolumeMap{volume.device: req.SizeInBytes}
			diskPlan.Add(types.Disks{&types.Disk{
				Device:    disk.Device,
				Mounts:    disk.Mounts,
				ReadIOPS:  req.ReadIOPS,
				WriteIOPS: req.WriteIOPS,
				ReadBPS:   req.ReadBPS,
				WriteBPS:  req.WriteBPS,
			}})
			allocated = true
			volumeToPush = append(volumeToPush, volume)
			break
		}

		// push the rest of the volumes back to the heap
		for _, volume := range volumeToPush {
			heap.Push(&volumeHeap, volume)
		}
		if !allocated {
			return nil, nil, coretypes.ErrInsufficientResource
		}
	}

	return volumePlan, diskPlan, nil
}

func (h *host) getNormalPlans(normalRequests types.VolumeBindings, mountRequests types.VolumeBindings) ([]types.VolumePlan, []types.Disks) {
	needScheduleMountRequest := utils.Any(mountRequests, func(req *types.VolumeBinding) bool { return req.RequireIOPS() })
	if len(normalRequests) == 0 && !needScheduleMountRequest {
		return utils.GenerateSlice(h.maxDeployCount, func() types.VolumePlan {
				return types.VolumePlan{}
			}), utils.GenerateSlice(h.maxDeployCount, func() types.Disks {
				return types.Disks{}
			})
	}

	volumePlans := []types.VolumePlan{}
	diskPlans := []types.Disks{}

	// h.usedVolumes is sorted by size, so we can allocate the volumes in order
	for {
		volumePlan, diskPlan, err := h.getNormalPlan(normalRequests)
		if err != nil {
			break
		}
		mountDiskPlan, err := h.getMountDiskPlan(mountRequests)
		if err != nil {
			break
		}
		diskPlan.Add(mountDiskPlan)
		volumePlans = append(volumePlans, volumePlan)
		diskPlans = append(diskPlans, diskPlan)
	}

	return volumePlans, diskPlans
}

func (h *host) getUnlimitedPlans(normalPlans, monoPlans []types.VolumePlan, unlimitedRequests types.VolumeBindings, capacity int) ([]types.VolumePlan, error) {
	if len(unlimitedRequests) == 0 {
		return utils.GenerateSlice(capacity, func() types.VolumePlan { return types.VolumePlan{} }), nil
	}
	volumes := append(h.usedVolumes.DeepCopy(), h.unusedVolumes.DeepCopy()...)
	if len(volumes) == 0 {
		return nil, coretypes.ErrInsufficientResource
	}
	volumeMap := map[string]*volume{}
	for _, volume := range volumes {
		volumeMap[volume.device] = volume
	}

	// apply normal plans and mono plans
	for _, plan := range append(normalPlans, monoPlans...) {
		for _, vm := range plan {
			volumeMap[vm.GetDevice()].size -= vm.GetSize()
		}
	}

	// get the volume with the largest size
	volumeWithLargestSize := volumes[0]
	for _, volume := range volumes {
		if volume.size > volumeWithLargestSize.size {
			volumeWithLargestSize = volume
		}
	}

	return utils.GenerateSlice(capacity, func() types.VolumePlan {
		volumePlan := types.VolumePlan{}
		for _, req := range unlimitedRequests {
			volumePlan[req] = types.VolumeMap{volumeWithLargestSize.device: req.SizeInBytes}
		}
		return volumePlan
	}), nil
}

func (h *host) classifyVolumeBindings(volumeBindings types.VolumeBindings) (normalRequests, monoRequests, unlimitedRequests, mountRequests types.VolumeBindings) {
	for _, binding := range volumeBindings {
		switch {
		case binding.RequireScheduleMonopoly():
			monoRequests = append(monoRequests, binding)
		case binding.RequireScheduleUnlimitedQuota():
			unlimitedRequests = append(unlimitedRequests, binding)
		case binding.RequireSchedule():
			normalRequests = append(normalRequests, binding)
		default:
			mountRequests = append(mountRequests, binding)
		}
	}

	sort.SliceStable(monoRequests, func(i, j int) bool { return monoRequests[i].SizeInBytes < monoRequests[j].SizeInBytes })
	sort.SliceStable(normalRequests, func(i, j int) bool { return normalRequests[i].SizeInBytes < normalRequests[j].SizeInBytes })

	return normalRequests, monoRequests, unlimitedRequests, mountRequests
}

// getMountDiskPlan will allocate IOPS quota for mount requests
func (h *host) getMountDiskPlan(reqs types.VolumeBindings) (types.Disks, error) {
	diskPlan := types.Disks{}
	for _, req := range reqs {
		disk := h.getDiskByPath(req.Source)
		if !h.isDiskIOPSQuotaQualified(disk, req) {
			return nil, coretypes.ErrInsufficientResource
		}
		h.decreaseIOPSQuota(disk, req)
		diskPlan.Add(types.Disks{&types.Disk{
			Device:    disk.Device,
			Mounts:    disk.Mounts,
			ReadIOPS:  req.ReadIOPS,
			WriteIOPS: req.WriteIOPS,
			ReadBPS:   req.ReadBPS,
			WriteBPS:  req.WriteBPS,
		}})
	}
	return diskPlan, nil
}

func (h *host) getVolumePlans(requests types.VolumeBindings) ([]types.VolumePlan, []types.Disks) {
	// if scheduling is not needed, return empty plans
	if !utils.Any(requests, func(req *types.VolumeBinding) bool {
		return req.RequireSchedule() || req.RequireIOPS()
	}) {
		return utils.GenerateSlice(h.maxDeployCount, func() types.VolumePlan {
				return types.VolumePlan{}
			}), utils.GenerateSlice(h.maxDeployCount, func() types.Disks {
				return types.Disks{}
			})
	}

	normalRequests, monoRequests, unlimitedRequests, mountRequests := h.classifyVolumeBindings(requests)
	if len(normalRequests)+len(monoRequests)+len(unlimitedRequests) > 0 && len(h.unusedVolumes)+len(h.usedVolumes) == 0 {
		return nil, nil
	}

	minNormalRequestSize := int64(math.MaxInt)
	for _, normalRequest := range normalRequests {
		if normalRequest.SizeInBytes < minNormalRequestSize {
			minNormalRequestSize = normalRequest.SizeInBytes
		}
	}

	// backup current state
	var (
		usedVolumesBackup, unusedVolumesBackup     volumes
		disksBackup                                types.Disks
		normalCapacity, monoCapacity, bestCapacity int
		bestVolumePlans                            [2][]types.VolumePlan
		bestDiskPlans                              [2][]types.Disks
	)

	backup := func() {
		usedVolumesBackup = h.usedVolumes.DeepCopy()
		unusedVolumesBackup = h.unusedVolumes.DeepCopy()
		disksBackup = h.disks.DeepCopy()
	}

	restore := func() {
		h.usedVolumes = usedVolumesBackup.DeepCopy()
		h.unusedVolumes = unusedVolumesBackup.DeepCopy()
		h.disks = disksBackup.DeepCopy()
	}

	getPlans := func() {
		normalVolumePlans, normalDiskPlans := h.getNormalPlans(normalRequests, mountRequests)
		monoVolumePlans, monoDiskPlans := h.getMonoPlans(monoRequests)
		normalCapacity = len(normalVolumePlans)
		monoCapacity = len(monoVolumePlans)
		bestCapacity = utils.Min(normalCapacity, monoCapacity)
		bestVolumePlans = [2][]types.VolumePlan{normalVolumePlans, monoVolumePlans}
		bestDiskPlans = [2][]types.Disks{normalDiskPlans, monoDiskPlans}
	}

	// get baseline
	backup()
	getPlans()
	restore()

	for monoCapacity > normalCapacity {
		// convert an unused volume to used volume
		p := sort.Search(len(h.unusedVolumes), func(i int) bool { return h.unusedVolumes[i].size >= minNormalRequestSize })
		// if no volume meets the min size requirement, just stop
		if p == len(h.unusedVolumes) {
			break
		}
		// move the unused volume to used volumes
		v := h.unusedVolumes[p]
		h.unusedVolumes = append(h.unusedVolumes[:p], h.unusedVolumes[p+1:]...)
		h.usedVolumes = append(h.usedVolumes, v)

		backup()
		getPlans()
		restore()
	}

	normalVolumePlans, monoVolumePlans := bestVolumePlans[0], bestVolumePlans[1]
	normalDiskPlans, monoDiskPlans := bestDiskPlans[0], bestDiskPlans[1]
	unlimitedVolumePlans, err := h.getUnlimitedPlans(normalVolumePlans, monoVolumePlans, unlimitedRequests, bestCapacity)
	if err != nil {
		log.Error(nil, err, "[getVolumePlans] failed to get unlimited volume plans") //nolint
		return nil, nil
	}

	// generate the final volume plans and disk plans
	resVolumePlans := utils.GenerateSlice(bestCapacity, func() types.VolumePlan { return types.VolumePlan{} })
	resDiskPlans := utils.GenerateSlice(bestCapacity, func() types.Disks { return types.Disks{} })

	for i := 0; i < bestCapacity; i++ {
		resVolumePlans[i] = normalVolumePlans[i]
		resVolumePlans[i].Merge(monoVolumePlans[i])
		resVolumePlans[i].Merge(unlimitedVolumePlans[i])
		resDiskPlans[i] = normalDiskPlans[i]
		resDiskPlans[i].Add(monoDiskPlans[i])
	}

	return resVolumePlans, resDiskPlans
}

func (h *host) getVolumeByDevice(device string) *volume {
	for _, volume := range append(h.usedVolumes, h.unusedVolumes...) {
		if volume.device == device {
			return volume
		}
	}
	return nil
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

func (h *host) getAffinityPlan(requests types.VolumeBindings, originVolumePlan types.VolumePlan, originRequests types.VolumeBindings) (types.VolumePlan, types.Disks, error) {
	// if scheduling is not needed, return empty plans
	if !utils.Any(requests, func(req *types.VolumeBinding) bool { return req.RequireSchedule() || req.RequireIOPS() }) {
		return types.VolumePlan{}, types.Disks{}, nil
	}

	// return all the resource (requested by the mount requests)
	_, _, _, oldMountRequests := h.classifyVolumeBindings(originRequests) //nolint
	for _, req := range oldMountRequests {
		if req.RequireIOPS() {
			disk := h.getDiskByPath(req.Source)
			if disk.Device == "" {
				err := errors.Wrapf(types.ErrInvalidVolume, "invalid path in the old mount requests: %s", req.Source)
				log.Errorf(nil, err, "[getAffinityPlan] invalid path in the old mount requests: %s", req.Source) //nolint
				return nil, nil, err
			}
			h.increaseIOPSQuota(disk, req)
		}
	}

	normalRequests, monoRequests, unlimitedRequests, mountRequests := h.classifyVolumeBindings(requests)
	needRescheduleRequests := types.VolumeBindings{}
	volumePlan := types.VolumePlan{}
	diskPlan := types.Disks{}

	// return all the remaining resource (requested by normal & mono requests)
	for req, volumeMap := range originVolumePlan {
		volume := h.getVolumeByDevice(volumeMap.GetDevice())
		if volume == nil {
			log.Errorf(nil, types.ErrInvalidVolume, "[getAffinityPlan] volume %s not found", volumeMap.GetDevice()) //nolint
			return nil, nil, types.ErrInvalidVolume
		}
		volume.size += volumeMap.GetSize()
		if req.RequireIOPS() {
			disk := h.getDiskByPath(volume.device)
			if disk.Device == "" {
				log.Errorf(nil, types.ErrInvalidVolume, "[getAffinityPlan] invalid path: %s", volume.device) //nolint
				return nil, nil, types.ErrInvalidVolume
			}
			h.increaseIOPSQuota(disk, req)
		}
	}

	commonProcess := func(requests types.VolumeBindings) error {
		affinity, nonAffinity := h.classifyAffinityRequests(requests, originVolumePlan)
		for req, volumeMap := range affinity {
			device := volumeMap.GetDevice()

			// check if the device has enough space
			volume := h.getVolumeByDevice(device)
			if req.SizeInBytes > volume.size {
				log.Errorf(nil, coretypes.ErrInsufficientResource, "[getAffinityPlan] no space to expand, %+v remains %+v, requires %+v", device, volume.size, req.SizeInBytes) //nolint
				return coretypes.ErrInsufficientResource
			}
			volume.size -= req.SizeInBytes
			volumePlan.Merge(types.VolumePlan{req: types.VolumeMap{volume.device: req.SizeInBytes}})

			// check if the disk has enough IOPS quota
			if !req.RequireIOPS() {
				continue
			}
			disk := h.getDiskByPath(device)
			if !h.isDiskIOPSQuotaQualified(disk, req) {
				log.Errorf(nil, coretypes.ErrInsufficientResource, "[getAffinityPlan] no IOPS quota to expand, %+v remains %+v, requires %+v", device, disk, req) //nolint
				return coretypes.ErrInsufficientResource
			}
			h.decreaseIOPSQuota(disk, req)
			diskPlan.Add(types.Disks{&types.Disk{
				Device:    disk.Device,
				Mounts:    disk.Mounts,
				ReadIOPS:  req.ReadIOPS,
				WriteIOPS: req.WriteIOPS,
				ReadBPS:   req.ReadBPS,
				WriteBPS:  req.WriteBPS,
			}})
		}
		for req := range nonAffinity {
			needRescheduleRequests = append(needRescheduleRequests, req)
		}
		return nil
	}

	// process mount requests
	mountDiskPlan, err := h.getMountDiskPlan(mountRequests)
	if err != nil {
		log.Error(nil, err, "[getAffinityPlan] alloc mount requests failed") //nolint
		return nil, nil, err
	}
	diskPlan.Add(mountDiskPlan)

	// process normal requests
	if err = commonProcess(normalRequests); err != nil {
		log.Error(nil, err, "[getAffinityPlan] alloc normal requests failed") //nolint
		return nil, nil, err
	}

	// process mono requests
	totalRequestSize := utils.Sum(utils.Map(monoRequests, func(req *types.VolumeBinding) int64 { return req.SizeInBytes }))
	totalVolumeSize := int64(0)
	for req, volumeMap := range originVolumePlan {
		if req.RequireScheduleMonopoly() {
			totalVolumeSize += volumeMap.GetSize()
		}
	}

	affinity, nonAffinity := h.classifyAffinityRequests(monoRequests, originVolumePlan)
	// if there is no affinity plan: all reschedule
	if len(affinity) == 0 {
		for req := range nonAffinity {
			needRescheduleRequests = append(needRescheduleRequests, req)
		}
	} else {
		// if there is any affinity plan: don't reschedule
		// use the first volume map to get the whole mono volume plan
		if totalVolumeSize < totalRequestSize { // check if the volume size is enough
			log.Errorf(nil, coretypes.ErrInsufficientResource, "[getAffinityPlan] no space to expand, the size of %+v is %+v, requires %+v", affinity[monoRequests[0]].GetDevice(), totalVolumeSize, totalRequestSize) //nolint
			return nil, nil, coretypes.ErrInsufficientResource
		}

		var volume *volume
		for _, volumeMap := range affinity {
			volume = h.getVolumeByDevice(volumeMap.GetDevice())
			if volume == nil {
				log.Errorf(nil, types.ErrInvalidVolume, "[getAffinityPlan] volume %s not found", volumeMap.GetDevice()) //nolint
				return nil, nil, types.ErrInvalidVolume
			}
			break
		}

		monoVolumePlan, monoDiskPlan, err := h.getMonoPlan(monoRequests, volume)
		if err != nil {
			log.Error(nil, err, "[getAffinityPlan] failed to get new mono plan") //nolint
			return nil, nil, err
		}

		volumePlan.Merge(monoVolumePlan)
		diskPlan.Add(types.Disks{monoDiskPlan})
	}

	// process unlimited requests
	if err := commonProcess(unlimitedRequests); err != nil {
		log.Error(nil, err, "[getAffinityPlan] alloc mount requests failed") //nolint
		return nil, nil, err
	}

	if len(needRescheduleRequests) == 0 {
		return volumePlan, diskPlan, nil
	}

	volumePlans, diskPlans := h.getVolumePlans(needRescheduleRequests)
	if len(volumePlans) == 0 {
		return nil, nil, coretypes.ErrInsufficientResource
	}
	volumePlan.Merge(volumePlans[0])
	diskPlan.Add(diskPlans[0])
	return volumePlan, diskPlan, nil
}

// GetAffinityPlan .
func GetAffinityPlan(resourceInfo *types.NodeResourceInfo, volumeRequest types.VolumeBindings, originVolumePlan types.VolumePlan, originVolumeRequest types.VolumeBindings) (types.VolumePlan, types.Disks, error) {
	h := newHost(resourceInfo, 1)
	return h.getAffinityPlan(volumeRequest, originVolumePlan, originVolumeRequest)
}

// GetVolumePlans .
func GetVolumePlans(resourceInfo *types.NodeResourceInfo, volumeRequest types.VolumeBindings, maxDeployCount int) ([]types.VolumePlan, []types.Disks) {
	h := newHost(resourceInfo, maxDeployCount)
	return h.getVolumePlans(volumeRequest)
}
