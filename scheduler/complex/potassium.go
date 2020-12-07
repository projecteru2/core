package complexscheduler

import (
	"sort"

	"math"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
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
		return nil, types.ErrInsufficientNodes
	}
	pos := 0
	node := nodes[pos]
	min := float64(node.CPU.Total())/float64(node.InitCPU.Total()) + float64(node.MemCap)/float64(node.InitMemCap)
	for i, node := range nodes {
		idle := float64(node.CPU.Total())/float64(node.InitCPU.Total()) + float64(node.MemCap)/float64(node.InitMemCap)
		if idle < min {
			pos = i
			min = idle
		}
	}
	return nodes[pos], nil
}

// SelectStorageNodes filters nodes with enough storage
func (m *Potassium) SelectStorageNodes(scheduleInfos []resourcetypes.ScheduleInfo, storage int64) ([]resourcetypes.ScheduleInfo, int, error) {
	switch {
	case storage < 0:
		return nil, 0, types.ErrNegativeStorage
	case storage == 0:
		return scheduleInfos, math.MaxInt64, nil
	default:
		log.Infof("[SelectStorageNodes] scheduleInfos: %v, need: %d", scheduleInfos, storage)
	}

	leng := len(scheduleInfos)

	sort.Slice(scheduleInfos, func(i, j int) bool { return scheduleInfos[i].StorageCap < scheduleInfos[j].StorageCap })
	p := sort.Search(leng, func(i int) bool { return scheduleInfos[i].StorageCap >= storage })
	if p == leng {
		return nil, 0, types.ErrInsufficientStorage
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
func (m *Potassium) SelectMemoryNodes(scheduleInfos []resourcetypes.ScheduleInfo, quota float64, memory int64) ([]resourcetypes.ScheduleInfo, int, error) {
	log.Infof("[SelectMemoryNodes] scheduleInfos: %v, need cpu: %f, memory: %d", scheduleInfos, quota, memory)
	scheduleInfosLength := len(scheduleInfos)

	// 筛选出能满足 CPU 需求的
	sort.Slice(scheduleInfos, func(i, j int) bool { return len(scheduleInfos[i].CPU) < len(scheduleInfos[j].CPU) })
	p := sort.Search(scheduleInfosLength, func(i int) bool {
		return float64(len(scheduleInfos[i].CPU)) >= quota
	})
	// p 最大也就是 scheduleInfosLength - 1
	if p == scheduleInfosLength {
		return nil, 0, types.ErrInsufficientCPU
	}
	scheduleInfosLength -= p
	scheduleInfos = scheduleInfos[p:]

	// 计算是否有足够的内存满足需求
	sort.Slice(scheduleInfos, func(i, j int) bool { return scheduleInfos[i].MemCap < scheduleInfos[j].MemCap })
	p = sort.Search(scheduleInfosLength, func(i int) bool { return scheduleInfos[i].MemCap >= memory })
	if p == scheduleInfosLength {
		return nil, 0, types.ErrInsufficientMEM
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
func (m *Potassium) SelectCPUNodes(scheduleInfos []resourcetypes.ScheduleInfo, quota float64, memory int64) ([]resourcetypes.ScheduleInfo, map[string][]types.CPUMap, int, error) {
	log.Infof("[SelectCPUNodes] scheduleInfos %d, need cpu: %f memory: %d", len(scheduleInfos), quota, memory)
	if quota <= 0 {
		return nil, nil, 0, types.ErrNegativeQuota
	}
	if len(scheduleInfos) == 0 {
		return nil, nil, 0, types.ErrZeroNodes
	}
	return cpuPriorPlan(quota, memory, scheduleInfos, m.maxshare, m.sharebase)
}

// SelectVolumeNodes calculates plans for volume request
func (m *Potassium) SelectVolumeNodes(scheduleInfos []resourcetypes.ScheduleInfo, vbs types.VolumeBindings) ([]resourcetypes.ScheduleInfo, map[string][]types.VolumePlan, int, error) {
	log.Infof("[SelectVolumeNodes] scheduleInfos %v, need volume: %v", scheduleInfos, vbs)
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
		return nil, nil, 0, types.ErrInsufficientRes
	}

	return scheduleInfos[p:], volumePlans, volTotal, nil
}
