package complexscheduler

import (
	"sort"

	"math"

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
func (m *Potassium) SelectStorageNodes(nodesInfo []types.NodeInfo, storage int64) ([]types.NodeInfo, int, error) {
	switch {
	case storage < 0:
		return nil, 0, types.ErrNegativeStorage
	case storage == 0:
		return nodesInfo, math.MaxInt32, nil
	default:
		log.Infof("[SelectStorageNodes] nodesInfo: %v, need: %d", nodesInfo, storage)
	}

	leng := len(nodesInfo)

	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].StorageCap < nodesInfo[j].StorageCap })
	p := sort.Search(leng, func(i int) bool { return nodesInfo[i].StorageCap >= storage })
	if p == leng {
		return nil, 0, types.ErrInsufficientStorage
	}

	nodesInfo = nodesInfo[p:]

	total := 0
	for i := range nodesInfo {
		storCap := int(nodesInfo[i].StorageCap / storage)
		nodesInfo[i].Capacity = utils.Min(storCap, nodesInfo[i].Capacity)
		total += nodesInfo[i].Capacity
	}

	return nodesInfo, total, nil
}

// SelectMemoryNodes filter nodes with enough memory
func (m *Potassium) SelectMemoryNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, int, error) {
	log.Infof("[SelectMemoryNodes] nodesInfo: %v, need cpu: %f, memory: %d", nodesInfo, quota, memory)
	if memory < 0 {
		return nil, 0, types.ErrNegativeMemory
	}
	nodesInfoLength := len(nodesInfo)

	// 筛选出能满足 CPU 需求的
	sort.Slice(nodesInfo, func(i, j int) bool { return len(nodesInfo[i].CPUMap) < len(nodesInfo[j].CPUMap) })
	p := sort.Search(nodesInfoLength, func(i int) bool {
		return float64(len(nodesInfo[i].CPUMap)) >= quota
	})
	// p 最大也就是 nodesInfoLength - 1
	if p == nodesInfoLength {
		return nil, 0, types.ErrInsufficientCPU
	}
	nodesInfoLength -= p
	nodesInfo = nodesInfo[p:]

	// 计算是否有足够的内存满足需求
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].MemCap < nodesInfo[j].MemCap })
	p = sort.Search(nodesInfoLength, func(i int) bool { return nodesInfo[i].MemCap >= memory })
	if p == nodesInfoLength {
		return nil, 0, types.ErrInsufficientMEM
	}
	nodesInfo = nodesInfo[p:]

	// 这里 memCap 一定是大于 memory 的所以不用判断 cap 内容
	volTotal := 0
	for i, nodeInfo := range nodesInfo {
		capacity := math.MaxInt16
		if memory != 0 {
			capacity = int(nodeInfo.MemCap / memory)
		}
		volTotal += capacity
		nodesInfo[i].Capacity = capacity
	}
	return nodesInfo, volTotal, nil
}

// SelectCPUNodes select nodes with enough cpus
func (m *Potassium) SelectCPUNodes(nodesInfo []types.NodeInfo, quota float64, memory int64) ([]types.NodeInfo, map[string][]types.CPUMap, int, error) {
	log.Infof("[SelectCPUNodes] nodesInfo %d, need cpu: %f memory: %d", len(nodesInfo), quota, memory)
	if quota <= 0 {
		return nil, nil, 0, types.ErrNegativeQuota
	}
	if memory < 0 {
		return nil, nil, 0, types.ErrNegativeMemory
	}
	if len(nodesInfo) == 0 {
		return nil, nil, 0, types.ErrZeroNodes
	}
	return cpuPriorPlan(quota, memory, nodesInfo, m.maxshare, m.sharebase)
}

// SelectVolumeNodes calculates plans for volume request
func (m *Potassium) SelectVolumeNodes(nodesInfo []types.NodeInfo, vbs types.VolumeBindings) ([]types.NodeInfo, map[string][]types.VolumePlan, int, error) {
	log.Infof("[SelectVolumeNodes] nodesInfo %v, need volume: %v", nodesInfo, vbs)
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

	volTotal := 0
	volumePlans := map[string][]types.VolumePlan{}
	for idx, nodeInfo := range nodesInfo {

		usedVolumeMap, unusedVolumeMap := nodeInfo.VolumeMap.SplitByUsed(nodeInfo.InitVolumeMap)
		if len(reqsMono) == 0 {
			usedVolumeMap.Add(unusedVolumeMap)
		}

		capNorm, plansNorm := calculateVolumePlan(usedVolumeMap, reqsNorm)
		capMono, plansMono := calculateMonopolyVolumePlan(nodeInfo.InitVolumeMap, unusedVolumeMap, reqsMono)

		volTotal += updateNodeInfoCapacity(&nodesInfo[idx], utils.Min(capNorm, capMono))
		cap := nodesInfo[idx].Capacity

		volumePlans[nodeInfo.Name] = make([]types.VolumePlan, cap)
		for idx := range volumePlans[nodeInfo.Name] {
			volumePlans[nodeInfo.Name][idx] = types.VolumePlan{}
		}
		if plansNorm != nil {
			for i, plan := range plansNorm[:cap] {
				volumePlans[nodeInfo.Name][i].Merge(types.MakeVolumePlan(vbsNorm, plan))
			}
		}
		if plansMono != nil {
			for i, plan := range plansMono[:cap] {
				volumePlans[nodeInfo.Name][i].Merge(types.MakeVolumePlan(vbsMono, plan))

			}
		}

		if len(vbsUnlimited) > 0 {
			// select the device with the most capacity as unlimited plan volume
			volume := types.VolumeMap{"": 0}
			currentMaxAvailable := int64(0)
			for vol, available := range nodeInfo.VolumeMap {
				if available > currentMaxAvailable {
					currentMaxAvailable = available
					volume = types.VolumeMap{vol: 0}
				}
			}

			planUnlimited := types.VolumePlan{}
			for _, vb := range vbsUnlimited {
				planUnlimited[*vb] = volume
			}

			for i := range volumePlans[nodeInfo.Name] {
				volumePlans[nodeInfo.Name][i].Merge(planUnlimited)
			}
		}
	}

	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Capacity < nodesInfo[j].Capacity })
	p := sort.Search(len(nodesInfo), func(i int) bool { return nodesInfo[i].Capacity > 0 })
	if p == len(nodesInfo) {
		return nil, nil, 0, types.ErrInsufficientRes
	}

	return nodesInfo[p:], volumePlans, volTotal, nil
}
