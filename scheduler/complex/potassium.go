package complexscheduler

import (
	"context"
	"math"
	"sort"

	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// Potassium is a scheduler
type Potassium struct {
	maxshare, sharebase int
}

// New a potassium
func New(config types.Config) (*Potassium, error) {
	return &Potassium{config.Scheduler.MaxShare, config.Scheduler.ShareBase}, nil
}

// MaxIdleNode use for build
func (m *Potassium) MaxIdleNode(nodes []*types.Node) (*types.Node, error) {
	if len(nodes) < 1 {
		return nil, errors.WithStack(types.ErrInsufficientNodes)
	}
	pos := 0
	node := nodes[pos]
	max := float64(node.CPU.Total())/float64(node.InitCPU.Total()) + float64(node.MemCap)/float64(node.InitMemCap)
	for i, node := range nodes {
		idle := float64(node.CPU.Total())/float64(node.InitCPU.Total()) + float64(node.MemCap)/float64(node.InitMemCap)
		if idle > max {
			pos = i
			max = idle
		}
	}
	return nodes[pos], nil
}

// SelectStorageNodes filters nodes with enough storage
func (m *Potassium) SelectStorageNodes(ctx context.Context, scheduleInfos []resourcetypes.ScheduleInfo, storage int64) ([]resourcetypes.ScheduleInfo, int, error) {
	switch {
	case storage < 0:
		return nil, 0, errors.WithStack(types.ErrNegativeStorage)
	case storage == 0:
		return scheduleInfos, math.MaxInt64, nil
	default:
		storages := []struct {
			Nodename string
			Storage  int64
		}{}
		for _, scheduleInfo := range scheduleInfos {
			storages = append(storages, struct {
				Nodename string
				Storage  int64
			}{scheduleInfo.Name, scheduleInfo.StorageCap})
		}
		log.Infof(ctx, "[SelectStorageNodes] resources: %v, need: %d", storages, storage)
	}

	leng := len(scheduleInfos)

	sort.Slice(scheduleInfos, func(i, j int) bool { return scheduleInfos[i].StorageCap < scheduleInfos[j].StorageCap })
	p := sort.Search(leng, func(i int) bool { return scheduleInfos[i].StorageCap >= storage })
	if p == leng {
		return nil, 0, errors.Wrapf(types.ErrInsufficientStorage, "no node remains storage more than %d bytes", storage)
	}

	scheduleInfos = scheduleInfos[p:]

	total := 0
	for i := range scheduleInfos {
		storCap := int(scheduleInfos[i].StorageCap / storage)
		total += updateScheduleInfoCapacity(&scheduleInfos[i], storCap)
	}

	return scheduleInfos, total, nil
}

// SelectMemoryNodes filter nodes with enough memory
func (m *Potassium) SelectMemoryNodes(ctx context.Context, scheduleInfos []resourcetypes.ScheduleInfo, quota float64, memory int64) ([]resourcetypes.ScheduleInfo, int, error) {
	resources := []struct {
		Nodename string
		CPU      types.CPUMap
		Memory   int64
	}{}
	for _, scheduleInfo := range scheduleInfos {
		resources = append(resources, struct {
			Nodename string
			CPU      types.CPUMap
			Memory   int64
		}{scheduleInfo.Name, scheduleInfo.CPU, scheduleInfo.MemCap})
	}
	log.Infof(ctx, "[SelectMemoryNodes] resources: %v, need cpu: %f, memory: %d", resources, quota, memory)
	scheduleInfosLength := len(scheduleInfos)

	// 筛选出能满足 CPU 需求的
	sort.Slice(scheduleInfos, func(i, j int) bool { return len(scheduleInfos[i].CPU) < len(scheduleInfos[j].CPU) })
	p := sort.Search(scheduleInfosLength, func(i int) bool {
		return float64(len(scheduleInfos[i].CPU)) >= quota
	})
	// p 最大也就是 scheduleInfosLength - 1
	if p == scheduleInfosLength {
		return nil, 0, errors.Wrapf(types.ErrInsufficientCPU, "no node remains cpu more than %0.2f", quota)
	}
	scheduleInfosLength -= p
	scheduleInfos = scheduleInfos[p:]

	// 计算是否有足够的内存满足需求
	sort.Slice(scheduleInfos, func(i, j int) bool { return scheduleInfos[i].MemCap < scheduleInfos[j].MemCap })
	p = sort.Search(scheduleInfosLength, func(i int) bool { return scheduleInfos[i].MemCap >= memory })
	if p == scheduleInfosLength {
		return nil, 0, errors.Wrapf(types.ErrInsufficientMEM, "no node remains memory more than %d bytes", memory)
	}
	scheduleInfos = scheduleInfos[p:]

	// 这里 memCap 一定是大于 memory 的所以不用判断 cap 内容
	volTotal := 0
	for i, scheduleInfo := range scheduleInfos {
		capacity := math.MaxInt32
		if memory != 0 {
			capacity = int(scheduleInfo.MemCap / memory)
		}
		volTotal += capacity
		scheduleInfos[i].Capacity = capacity
	}
	return scheduleInfos, volTotal, nil
}

// SelectCPUNodes select nodes with enough cpus
func (m *Potassium) SelectCPUNodes(ctx context.Context, scheduleInfos []resourcetypes.ScheduleInfo, quota float64, memory int64) ([]resourcetypes.ScheduleInfo, map[string][]types.CPUMap, int, error) {
	resources := []struct {
		Nodename string
		Memory   int64
		CPU      types.CPUMap
	}{}
	for _, scheduleInfo := range scheduleInfos {
		resources = append(resources, struct {
			Nodename string
			Memory   int64
			CPU      types.CPUMap
		}{scheduleInfo.Name, scheduleInfo.MemCap, scheduleInfo.CPU})
	}
	log.Infof(ctx, "[SelectCPUNodes] resources %v, need cpu: %f memory: %d", resources, quota, memory)
	if quota <= 0 {
		return nil, nil, 0, errors.WithStack(types.ErrNegativeCPU)
	}
	if len(scheduleInfos) == 0 {
		return nil, nil, 0, errors.WithStack(types.ErrZeroNodes)
	}

	return cpuPriorPlan(ctx, quota, memory, scheduleInfos, m.maxshare, m.sharebase)
}

// ReselectCPUNodes used for realloc one container with cpu affinity
func (m *Potassium) ReselectCPUNodes(ctx context.Context, scheduleInfo resourcetypes.ScheduleInfo, CPU types.CPUMap, quota float64, memory int64) (resourcetypes.ScheduleInfo, map[string][]types.CPUMap, int, error) {
	log.Infof(ctx, "[ReselectCPUNodes] resources %v, need cpu %f, need memory %d, existing %v",
		struct {
			Nodename string
			Memory   int64
			CPU      types.CPUMap
		}{scheduleInfo.Name, scheduleInfo.MemCap, scheduleInfo.CPU},
		quota, memory, CPU)
	var affinityPlan types.CPUMap
	// remaining quota that's impossible to achieve affinity
	if scheduleInfo, quota, affinityPlan = cpuReallocPlan(scheduleInfo, quota, CPU, int64(m.sharebase)); quota == 0 {
		cpuPlans := map[string][]types.CPUMap{
			scheduleInfo.Name: {
				affinityPlan,
			},
		}
		scheduleInfo.Capacity = 1
		return scheduleInfo, cpuPlans, 1, nil
	}

	scheduleInfos, cpuPlans, total, err := m.SelectCPUNodes(ctx, []resourcetypes.ScheduleInfo{scheduleInfo}, quota, memory)
	if err != nil {
		return scheduleInfo, nil, 0, errors.Wrap(err, "failed to reschedule cpu")
	}

	// add affinity plans
	for i, plan := range cpuPlans[scheduleInfo.Name] {
		for cpuID, pieces := range affinityPlan {
			if _, ok := plan[cpuID]; ok {
				cpuPlans[scheduleInfo.Name][i][cpuID] += pieces
			} else {
				cpuPlans[scheduleInfo.Name][i][cpuID] = pieces
			}
		}
	}
	return scheduleInfos[0], cpuPlans, total, nil
}

func cpuReallocPlan(scheduleInfo resourcetypes.ScheduleInfo, quota float64, CPU types.CPUMap, sharebase int64) (resourcetypes.ScheduleInfo, float64, types.CPUMap) {
	affinityPlan := make(types.CPUMap)
	diff := int64(quota*float64(sharebase)) - CPU.Total()
	// sort by pieces
	cpuIDs := []string{}
	for cpuID := range CPU {
		cpuIDs = append(cpuIDs, cpuID)
	}
	sort.Slice(cpuIDs, func(i, j int) bool { return CPU[cpuIDs[i]] < CPU[cpuIDs[j]] })

	// shrink, ensure affinity
	if diff <= 0 {
		affinityPlan = CPU
		// prioritize fragments
		for _, cpuID := range cpuIDs {
			if diff == 0 {
				break
			}
			shrink := utils.Min(affinityPlan[cpuID], -diff)
			affinityPlan[cpuID] -= shrink
			if affinityPlan[cpuID] == 0 {
				delete(affinityPlan, cpuID)
			}
			diff += shrink
			scheduleInfo.CPU[cpuID] += shrink
		}
		return scheduleInfo, float64(0), affinityPlan
	}

	// expand, prioritize full cpus
	needPieces := int64(quota * float64(sharebase))
	for i := len(cpuIDs) - 1; i >= 0; i-- {
		cpuID := cpuIDs[i]
		if needPieces == 0 {
			scheduleInfo.CPU[cpuID] += CPU[cpuID]
			continue
		}

		// whole cpu, keep it
		if CPU[cpuID] == sharebase {
			affinityPlan[cpuID] = sharebase
			needPieces -= sharebase
			continue
		}

		// fragments, try to find complement
		if available := scheduleInfo.CPU[cpuID]; available == sharebase-CPU[cpuID] {
			expand := utils.Min(available, needPieces)
			affinityPlan[cpuID] = CPU[cpuID] + expand
			scheduleInfo.CPU[cpuID] -= expand
			needPieces -= sharebase
			continue
		}

		// else, return to cpu pools
		scheduleInfo.CPU[cpuID] += CPU[cpuID]
	}

	return scheduleInfo, float64(needPieces) / float64(sharebase), affinityPlan
}

// SelectVolumeNodes calculates plans for volume request
func (m *Potassium) SelectVolumeNodes(ctx context.Context, scheduleInfos []resourcetypes.ScheduleInfo, vbs types.VolumeBindings) ([]resourcetypes.ScheduleInfo, map[string][]types.VolumePlan, int, error) {
	resources := []struct {
		Nodename string
		Volume   types.VolumeMap
	}{}
	for _, scheduleInfo := range scheduleInfos {
		resources = append(resources, struct {
			Nodename string
			Volume   types.VolumeMap
		}{scheduleInfo.Name, scheduleInfo.Volume})
	}
	log.Infof(ctx, "[SelectVolumeNodes] resources %v, need volume: %v", resources, vbs.ToStringSlice(true, true))

	var reqsNorm, reqsMono []int64
	var vbsNorm, vbsMono, vbsUnlimited types.VolumeBindings

	for _, vb := range vbs {
		switch {
		case vb.RequireScheduleMonopoly():
			vbsMono = append(vbsMono, vb)
			reqsMono = append(reqsMono, vb.SizeInBytes)
		case vb.RequireScheduleUnlimitedQuota():
			vbsUnlimited = append(vbsUnlimited, vb)
		case vb.RequireSchedule():
			vbsNorm = append(vbsNorm, vb)
			reqsNorm = append(reqsNorm, vb.SizeInBytes)
		}
	}

	if len(vbsNorm) == 0 && len(vbsMono) == 0 && len(vbsUnlimited) == 0 {
		for i := range scheduleInfos {
			scheduleInfos[i].Capacity = math.MaxInt64
		}
		return scheduleInfos, nil, math.MaxInt64, nil
	}

	volTotal := 0
	volumePlans := map[string][]types.VolumePlan{}
	for idx, scheduleInfo := range scheduleInfos {
		if len(scheduleInfo.Volume) == 0 {
			volTotal += updateScheduleInfoCapacity(&scheduleInfos[idx], 0)
			continue
		}

		usedVolumeMap, unusedVolumeMap := scheduleInfo.Volume.SplitByUsed(scheduleInfo.InitVolume)
		if len(reqsMono) == 0 {
			usedVolumeMap.Add(unusedVolumeMap)
		}
		if len(reqsNorm) != 0 && len(usedVolumeMap) == 0 && len(unusedVolumeMap) != 0 {
			usedVolumeMap = types.VolumeMap{}
			// give out half of volumes
			half, cnt, toDelete := (len(unusedVolumeMap)+1)/2, 0, []string{}
			for i, v := range unusedVolumeMap {
				cnt++
				if cnt > half {
					break
				}
				toDelete = append(toDelete, i)
				usedVolumeMap[i] = v
			}
			for _, i := range toDelete {
				delete(unusedVolumeMap, i)
			}
		}

		capNorm, plansNorm := calculateVolumePlan(usedVolumeMap, reqsNorm)
		capMono, plansMono := calculateMonopolyVolumePlan(scheduleInfo.InitVolume, unusedVolumeMap, reqsMono)

		volTotal += updateScheduleInfoCapacity(&scheduleInfos[idx], utils.Min(capNorm, capMono))
		cap := scheduleInfos[idx].Capacity

		volumePlans[scheduleInfo.Name] = make([]types.VolumePlan, cap)
		for idx := range volumePlans[scheduleInfo.Name] {
			volumePlans[scheduleInfo.Name][idx] = types.VolumePlan{}
		}
		if plansNorm != nil {
			for i, plan := range plansNorm[:cap] {
				volumePlans[scheduleInfo.Name][i].Merge(types.MakeVolumePlan(vbsNorm, plan))
			}
		}
		if plansMono != nil {
			for i, plan := range plansMono[:cap] {
				volumePlans[scheduleInfo.Name][i].Merge(types.MakeVolumePlan(vbsMono, plan))

			}
		}

		if len(vbsUnlimited) > 0 {
			// select the device with the most capacity as unlimited plan volume
			volume := types.VolumeMap{"": 0}
			currentMaxAvailable := int64(0)
			for vol, available := range scheduleInfo.Volume {
				if available > currentMaxAvailable {
					currentMaxAvailable = available
					volume = types.VolumeMap{vol: 0}
				}
			}

			planUnlimited := types.VolumePlan{}
			for _, vb := range vbsUnlimited {
				planUnlimited[*vb] = volume
			}

			for i := range volumePlans[scheduleInfo.Name] {
				volumePlans[scheduleInfo.Name][i].Merge(planUnlimited)
			}
		}
	}

	sort.Slice(scheduleInfos, func(i, j int) bool { return scheduleInfos[i].Capacity < scheduleInfos[j].Capacity })
	p := sort.Search(len(scheduleInfos), func(i int) bool { return scheduleInfos[i].Capacity > 0 })
	if p == len(scheduleInfos) {
		return nil, nil, 0, errors.Wrapf(types.ErrInsufficientRes, "no node remains volumes for requests %+v", vbs.ToStringSlice(true, true))
	}

	return scheduleInfos[p:], volumePlans, volTotal, nil
}

// ReselectVolumeNodes is used for realloc only
func (m *Potassium) ReselectVolumeNodes(ctx context.Context, scheduleInfo resourcetypes.ScheduleInfo, existing types.VolumePlan, vbsReq types.VolumeBindings) (resourcetypes.ScheduleInfo, map[string][]types.VolumePlan, int, error) {
	log.Infof(ctx, "[ReselectVolumeNodes] resources: %v, need volume: %v, existing %v",
		struct {
			Nodename   string
			Volume     types.VolumeMap
			InitVolume types.VolumeMap
		}{scheduleInfo.Name, scheduleInfo.Volume, scheduleInfo.InitVolume},
		vbsReq.ToStringSlice(true, true), existing.ToLiteral())
	affinityPlan := types.VolumePlan{}
	needReschedule := types.VolumeBindings{}
	norm, mono, unlim := distinguishVolumeBindings(vbsReq)

	// norm
	normAff, normRem := distinguishAffinityVolumeBindings(norm, existing)
	needReschedule = append(needReschedule, normRem...)
	for _, vb := range normAff {
		_, oldVM, _ := existing.FindAffinityPlan(*vb)
		if scheduleInfo.Volume[oldVM.GetResourceID()] < vb.SizeInBytes {
			return scheduleInfo, nil, 0, errors.Wrapf(types.ErrInsufficientVolume, "no space to expand: %+v, %+v", oldVM, vb)
		}
		affinityPlan.Merge(types.VolumePlan{
			*vb: types.VolumeMap{oldVM.GetResourceID(): vb.SizeInBytes},
		})
		scheduleInfo.Volume[oldVM.GetResourceID()] -= vb.SizeInBytes
	}

	// mono
	monoAff, _ := distinguishAffinityVolumeBindings(mono, existing)
	if len(monoAff) == 0 {
		// all reschedule
		needReschedule = append(needReschedule, mono...)

	} else {
		// all no reschedule
		_, oldVM, _ := existing.FindAffinityPlan(*monoAff[0])
		monoVolume := oldVM.GetResourceID()
		newVms, monoTotal, newMonoPlan := []types.VolumeMap{}, int64(0), types.VolumePlan{}
		for _, vb := range mono {
			monoTotal += vb.SizeInBytes
			newVms = append(newVms, types.VolumeMap{monoVolume: vb.SizeInBytes})
		}
		if monoTotal > scheduleInfo.InitVolume[monoVolume] {
			return scheduleInfo, nil, 0, errors.Wrap(types.ErrInsufficientVolume, "no space to expand mono volumes: ")
		}
		newVms = proportionPlan(newVms, scheduleInfo.InitVolume[monoVolume])
		for i, vb := range mono {
			newMonoPlan[*vb] = newVms[i]
		}
		affinityPlan.Merge(newMonoPlan)
		scheduleInfo.Volume[monoVolume] = 0
	}

	// unlimit
	unlimAff, unlimRem := distinguishAffinityVolumeBindings(unlim, existing)
	needReschedule = append(needReschedule, unlimRem...)
	unlimPlan := types.VolumePlan{}
	for _, vb := range unlimAff {
		_, oldVM, _ := existing.FindAffinityPlan(*vb)
		unlimPlan[*vb] = oldVM
	}
	affinityPlan.Merge(unlimPlan)

	// schedule new volume requests
	if len(needReschedule) == 0 {
		scheduleInfo.Capacity = 1
		return scheduleInfo, map[string][]types.VolumePlan{scheduleInfo.Name: {affinityPlan}}, 1, nil
	}
	scheduleInfos, volumePlans, total, err := m.SelectVolumeNodes(ctx, []resourcetypes.ScheduleInfo{scheduleInfo}, needReschedule)
	if err != nil {
		return scheduleInfo, nil, 0, errors.Wrap(err, "failed to reschedule volume")
	}

	// merge
	for i := range volumePlans[scheduleInfo.Name] {
		volumePlans[scheduleInfo.Name][i].Merge(affinityPlan)
	}

	return scheduleInfos[0], volumePlans, total, nil
}
