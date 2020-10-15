package types

import (
	"context"
	"encoding/json"
	"sort"

	"math"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

const (
	// IncrUsage add cpuusage
	IncrUsage = "+"
	// DecrUsage cpuusage
	DecrUsage = "-"
	// AUTO indicates that volume is to be scheduled by scheduler
	AUTO = "AUTO"
)

// ResourceMap is cpu core map
// ResourceMap {["0"]10000, ["1"]10000}
type ResourceMap map[string]int64

// Total show total cpu
// Total quotas
func (c ResourceMap) Total() int64 {
	var count int64
	for _, value := range c {
		count += value
	}
	return count
}

// Add return cpu
func (c ResourceMap) Add(q ResourceMap) {
	for label, value := range q {
		if _, ok := c[label]; !ok {
			c[label] = value
		} else {
			c[label] += value
		}
	}
}

// Sub decrease cpus
func (c ResourceMap) Sub(q ResourceMap) {
	for label, value := range q {
		if _, ok := c[label]; ok {
			c[label] -= value
		}
	}
}

// CPUMap is cpu core map
// CPUMap {["0"]10000, ["1"]10000}
type CPUMap = ResourceMap

// VolumeMap is volume map
// VolumeMap {["/data1"]1073741824, ["/data2"]1048576}
type VolumeMap = ResourceMap

// GetResourceID returns device name such as "/sda0"
// GetResourceID only works for VolumeMap with single key
func (c VolumeMap) GetResourceID() (key string) {
	for k := range c {
		key = k
		break
	}
	return
}

// GetRation returns scheduled size from device
// GetRation only works for VolumeMap with single key
func (c VolumeMap) GetRation() int64 {
	return c[c.GetResourceID()]
}

// SplitByUsed .
func (c VolumeMap) SplitByUsed(init VolumeMap) (VolumeMap, VolumeMap) {
	used := VolumeMap{}
	unused := VolumeMap{}
	for mountDir, freeSpace := range c {
		vmap := used
		if init[mountDir] == freeSpace {
			vmap = unused
		}
		vmap.Add(VolumeMap{mountDir: freeSpace})
	}
	return used, unused
}

// VolumePlan is map from volume string to volumeMap: {"AUTO:/data:rw:100": VolumeMap{"/sda1": 100}}
type VolumePlan map[VolumeBinding]VolumeMap

// MakeVolumePlan creates VolumePlan pointer by volume strings and scheduled VolumeMaps
func MakeVolumePlan(vbs VolumeBindings, distribution []VolumeMap) VolumePlan {
	sort.Slice(vbs, func(i, j int) bool { return vbs[i].SizeInBytes < vbs[j].SizeInBytes })
	sort.Slice(distribution, func(i, j int) bool { return distribution[i].GetRation() < distribution[j].GetRation() })

	volumePlan := VolumePlan{}
	for idx, vb := range vbs {
		if vb.RequireSchedule() {
			volumePlan[*vb] = distribution[idx]
		}
	}
	return volumePlan
}

// UnmarshalJSON .
func (p *VolumePlan) UnmarshalJSON(b []byte) (err error) {
	plan := map[string]VolumeMap{}
	if err = json.Unmarshal(b, &plan); err != nil {
		return err
	}
	for volume, vmap := range plan {
		vb, err := NewVolumeBinding(volume)
		if err != nil {
			return err
		}
		(*p)[*vb] = vmap
	}
	return
}

// MarshalJSON .
func (p VolumePlan) MarshalJSON() ([]byte, error) {
	plan := map[string]VolumeMap{}
	for vb, vmap := range p {
		plan[vb.ToString(false)] = vmap
	}
	return json.Marshal(plan)
}

// ToLiteral returns literal VolumePlan
func (p VolumePlan) ToLiteral() map[string]map[string]int64 {
	plan := map[string]map[string]int64{}
	for vb, volumeMap := range p {
		plan[vb.ToString(false)] = volumeMap
	}
	return plan
}

// IntoVolumeMap Merge return one VolumeMap with all in VolumePlan added
func (p VolumePlan) IntoVolumeMap() VolumeMap {
	volumeMap := VolumeMap{}
	for _, v := range p {
		volumeMap.Add(v)
	}
	return volumeMap
}

// GetVolumeMap looks up VolumeMap according to volume destination directory
func (p VolumePlan) GetVolumeMap(vb *VolumeBinding) (volMap VolumeMap, volume VolumeBinding) {
	for volume, volMap := range p {
		if vb.Destination == volume.Destination {
			return volMap, volume
		}
	}
	return
}

// Compatible return true if new bindings stick to the old bindings
func (p VolumePlan) Compatible(oldPlan VolumePlan) bool {
	for volume, oldBinding := range oldPlan {
		newBinding, _ := p.GetVolumeMap(&volume) // nolint
		// newBinding is ok to be nil when reallocing requires less volumes than before
		if newBinding != nil && newBinding.GetResourceID() != oldBinding.GetResourceID() {
			// unlimited binding, modify binding source
			if newBinding.GetRation() == 0 {
				//p[v] = VolumeMap{oldBinding.GetResourceID(): 0}
				continue
			}
			return false
		}
	}
	return true
}

// Merge .
func (p VolumePlan) Merge(p2 VolumePlan) {
	for vb, vm := range p2 {
		p[vb] = vm
	}
}

// NUMA define NUMA cpuID->nodeID
type NUMA map[string]string

// NUMAMemory fine NUMA memory NODE
type NUMAMemory map[string]int64

// Node store node info
type Node struct {
	Name     string `json:"name"`
	Endpoint string `json:"endpoint"`
	Podname  string `json:"podname"`
	CPU      CPUMap `json:"cpu"`
	// free spaces
	Volume         VolumeMap         `json:"volume"`
	NUMA           NUMA              `json:"numa"`
	NUMAMemory     NUMAMemory        `json:"numa_memory"`
	CPUUsed        float64           `json:"cpuused"`
	VolumeUsed     int64             `json:"volumeused"`
	MemCap         int64             `json:"memcap"`
	StorageCap     int64             `json:"storage_cap"`
	Available      bool              `json:"available"`
	Labels         map[string]string `json:"labels"`
	InitCPU        CPUMap            `json:"init_cpu"`
	InitMemCap     int64             `json:"init_memcap"`
	InitStorageCap int64             `json:"init_storage_cap"`
	InitNUMAMemory NUMAMemory        `json:"init_numa_memory"`
	InitVolume     VolumeMap         `json:"init_volume"`
	Engine         engine.API        `json:"-"`
}

// Init .
func (n *Node) Init() {
	if n.Volume == nil {
		n.Volume = VolumeMap{}
	}
	if n.InitVolume == nil {
		n.InitVolume = VolumeMap{}
	}
}

// Info show node info
func (n *Node) Info(ctx context.Context) (*enginetypes.Info, error) {
	if n.Engine == nil {
		return nil, ErrNilEngine
	}
	return n.Engine.Info(ctx)
}

// SetCPUUsed set cpuusage
func (n *Node) SetCPUUsed(quota float64, action string) {
	switch action {
	case IncrUsage:
		n.CPUUsed = Round(n.CPUUsed + quota)
	case DecrUsage:
		n.CPUUsed = Round(n.CPUUsed - quota)
	default:
	}
}

// SetVolumeUsed .
func (n *Node) SetVolumeUsed(cost int64, action string) {
	switch action {
	case IncrUsage:
		n.VolumeUsed += cost
	case DecrUsage:
		n.VolumeUsed -= cost
	default:
	}
}

// GetNUMANode get numa node
func (n *Node) GetNUMANode(cpu CPUMap) string {
	nodeID := ""
	for cpuID := range cpu {
		if memoryNode, ok := n.NUMA[cpuID]; ok {
			if nodeID == "" {
				nodeID = memoryNode
			} else if nodeID != memoryNode { // 如果跨 NODE 了，让系统决定 nodeID
				nodeID = ""
			}
		}
	}
	return nodeID
}

// IncrNUMANodeMemory set numa node memory
func (n *Node) IncrNUMANodeMemory(nodeID string, memory int64) {
	if _, ok := n.NUMAMemory[nodeID]; ok {
		n.NUMAMemory[nodeID] += memory
	}
}

// DecrNUMANodeMemory set numa node memory
func (n *Node) DecrNUMANodeMemory(nodeID string, memory int64) {
	if _, ok := n.NUMAMemory[nodeID]; ok {
		n.NUMAMemory[nodeID] -= memory
	}
}

// StorageUsage calculates node's storage usage ratio.
func (n *Node) StorageUsage() float64 {
	switch {
	case n.InitStorageCap <= 0:
		return 1.0
	default:
		return 1.0 - float64(n.StorageCap)/float64(n.InitStorageCap)
	}
}

// StorageUsed calculates node's storage usage value.
func (n *Node) StorageUsed() int64 {
	switch {
	case n.InitStorageCap <= 0:
		return 0
	default:
		return n.InitStorageCap - n.StorageCap
	}
}

// AvailableStorage calculates available value.
func (n *Node) AvailableStorage() int64 {
	switch {
	case n.InitStorageCap <= 0:
		return math.MaxInt64
	default:
		return n.StorageCap
	}
}

// ResourceUsages .
func (n *Node) ResourceUsages() map[ResourceType]float64 {
	return map[ResourceType]float64{
		ResourceCPU:     n.CPUUsed / float64(len(n.InitCPU)),
		ResourceMemory:  1.0 - float64(n.MemCap)/float64(n.InitMemCap),
		ResourceStorage: n.StorageUsage(),
		ResourceVolume:  float64(n.VolumeUsed) / float64(n.InitVolume.Total()),
	}
}

// NodeInfo for deploy
type NodeInfo struct {
	Name          string
	CPUMap        CPUMap
	VolumeMap     VolumeMap
	InitVolumeMap VolumeMap
	NUMA          NUMA
	NUMAMemory    NUMAMemory
	MemCap        int64
	StorageCap    int64

	Usages map[ResourceType]float64 // deprecated
	Rates  map[ResourceType]float64 // deprecated

	CPUPlan     []CPUMap
	VolumePlans []VolumePlan // {{"AUTO:/data:rw:1024": "/mnt0:/data:rw:1024"}}
	Capacity    int          // 可以部署几个, deprecated
	Count       int          // 上面有几个了, deprecated
	Deploy      int          // 最终部署几个, deprecated
	// 其他需要 filter 的字段
}

// NodeResource for node check
type NodeResource struct {
	Name              string
	CPU               CPUMap
	MemCap            int64
	StorageCap        int64
	CPUPercent        float64
	MemoryPercent     float64
	StoragePercent    float64
	NUMAMemoryPercent map[string]float64
	VolumePercent     float64
	Verification      bool
	Details           []string
	Containers        []*Container
}
