package types

import (
	"context"
	"encoding/json"
	"math"

	"github.com/pkg/errors"

	engine "github.com/projecteru2/core/engine"
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

	Ca   string `json:"-"`
	Cert string `json:"-"`
	Key  string `json:"-"`
}

// DeepCopy returns a deepcopy of nodemeta
func (n NodeMeta) DeepCopy() (nn NodeMeta, err error) {
	b, err := json.Marshal(n)
	if err != nil {
		return nn, errors.WithStack(err)
	}
	return nn, errors.WithStack(json.Unmarshal(b, &nn))
}

// Node store node info
type Node struct {
	NodeMeta
	NodeInfo string `json:"-"`

	CPUUsed    float64 `json:"cpuused"`
	VolumeUsed int64   `json:"volumeused"`

	// Bypass if bypass is true, it will not participate in future scheduling
	Bypass    bool       `json:"bypass,omitempty"`
	Available bool       `json:"-"`
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
func (n *Node) Info(ctx context.Context) (err error) {
	info, err := n.Engine.Info(ctx)
	if err != nil {
		n.Available = false
		n.NodeInfo = err.Error()
		return errors.WithStack(err)
	}
	bs, err := json.Marshal(info)
	if err != nil {
		n.NodeInfo = err.Error()
		return errors.WithStack(err)
	}
	n.NodeInfo = string(bs)
	return nil
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
	res := map[ResourceType]float64{
		ResourceCPU:     n.CPUUsed / float64(len(n.InitCPU)),
		ResourceMemory:  1.0 - float64(n.MemCap)/float64(n.InitMemCap),
		ResourceStorage: n.StorageUsage(),
		ResourceVolume:  float64(n.VolumeUsed) / float64(n.InitVolume.Total()),
	}
	for k, v := range res {
		if v > 1.0 {
			res[k] = 1.0
		}
		if v < 0 {
			res[k] = 0
		}
	}
	return res
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

// IsDown returns if the node is marked as down.
func (n *Node) IsDown() bool {
	// If `bypass` is true, then even if the node is still healthy, the node will be regarded as `down`.
	// Currently `bypass` will only be set when the cli calls the `up` and `down` commands.
	return n.Bypass || !n.Available
}

// NodeMetrics used for metrics collecting
type NodeMetrics struct {
	Name        string
	Podname     string
	Memory      float64
	MemoryUsed  float64
	Storage     float64
	StorageUsed float64
	CPUUsed     float64
	CPU         CPUMap
}

// Metrics reports metrics value
func (n *Node) Metrics() *NodeMetrics {
	nc := CPUMap{}
	for k, v := range n.CPU {
		nc[k] = v
	}
	return &NodeMetrics{
		Name:        n.Name,
		Podname:     n.Podname,
		Memory:      float64(n.MemCap),
		MemoryUsed:  float64(n.InitMemCap - n.MemCap),
		Storage:     float64(n.StorageCap),
		StorageUsed: float64(n.InitStorageCap - n.StorageCap),
		CPUUsed:     n.CPUUsed,
		CPU:         nc,
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

// NodeStatus wraps node status
// only used for node status stream
type NodeStatus struct {
	Nodename string
	Podname  string
	Alive    bool
	Error    error
}

// NodeFilter is used to filter nodes in a pod
// names in includes will be used
// names in excludes will not be used
type NodeFilter struct {
	Podname  string
	Includes []string
	Excludes []string
	Labels   map[string]string
	All      bool
}
