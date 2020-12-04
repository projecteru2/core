package types

import (
	"context"

	"math"

	engine "github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
)

const (
	// IncrUsage add cpuusage
	IncrUsage = "+"
	// DecrUsage cpuusage
	DecrUsage = "-"
)

// NUMA define NUMA cpuID->nodeID
type NUMA map[string]string

// NUMAMemory fine NUMA memory NODE
type NUMAMemory map[string]int64

// NodeMeta .
type NodeMeta struct {
	Name     string            `json:"name"`
	Endpoint string            `json:"endpoint"`
	Podname  string            `json:"podname"`
	Labels   map[string]string `json:"labels"`

	CPU            CPUMap     `json:"cpu"`
	Volume         VolumeMap  `json:"volume"`
	NUMA           NUMA       `json:"numa"`
	NUMAMemory     NUMAMemory `json:"numa_memory"`
	MemCap         int64      `json:"memcap"`
	StorageCap     int64      `json:"storage_cap"`
	InitCPU        CPUMap     `json:"init_cpu"`
	InitMemCap     int64      `json:"init_memcap"`
	InitStorageCap int64      `json:"init_storage_cap"`
	InitNUMAMemory NUMAMemory `json:"init_numa_memory"`
	InitVolume     VolumeMap  `json:"init_volume"`
}

// Node store node info
type Node struct {
	NodeMeta

	CPUUsed    float64 `json:"cpuused"`
	VolumeUsed int64   `json:"volumeused"`

	Available bool       `json:"available"`
	Engine    engine.API `json:"-"`
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

// RecycleResources .
func (n *Node) RecycleResources(resource *ResourceMeta) {
	n.CPU.Add(resource.CPU)
	n.SetCPUUsed(resource.CPUQuotaRequest, DecrUsage)
	n.Volume.Add(resource.VolumePlanRequest.IntoVolumeMap())
	n.SetVolumeUsed(resource.VolumePlanRequest.IntoVolumeMap().Total(), DecrUsage)
	n.MemCap += resource.MemoryRequest
	n.StorageCap += resource.StorageRequest
	if resource.NUMANode != "" {
		n.IncrNUMANodeMemory(resource.NUMANode, resource.MemoryRequest)
	}
}

// PreserveResources .
func (n *Node) PreserveResources(resource *ResourceMeta) {
	n.CPU.Sub(resource.CPU)
	n.SetCPUUsed(resource.CPUQuotaRequest, IncrUsage)
	n.Volume.Sub(resource.VolumePlanRequest.IntoVolumeMap())
	n.SetVolumeUsed(resource.VolumePlanRequest.IntoVolumeMap().Total(), IncrUsage)
	n.MemCap -= resource.MemoryRequest
	n.StorageCap -= resource.StorageRequest
	if resource.NUMANode != "" {
		n.DecrNUMANodeMemory(resource.NUMANode, resource.MemoryRequest)
	}
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
	Diffs             []string
	Workloads         []*Workload
}
