package types

import (
	"encoding/json"
	"sort"
)

// ResourceOptions for create/realloc/replace
type ResourceOptions struct {
	CPUQuotaRequest float64
	CPUQuotaLimit   float64
	CPUBind         bool
	CPU             CPUMap

	MemoryRequest int64
	MemoryLimit   int64

	VolumeRequest VolumeBindings
	VolumeLimit   VolumeBindings

	StorageRequest int64
	StorageLimit   int64
}

// ResourceMeta for messages and workload to store
type ResourceMeta struct {
	CPUQuotaRequest float64     `json:"cpu_quota_request"`
	CPUQuotaLimit   float64     `json:"cpu_quota_limit"`
	CPU             ResourceMap `json:"cpu"`
	NUMANode        string      `json:"numa_node"`

	MemoryRequest int64 `json:"memory_request"`
	MemoryLimit   int64 `json:"memory_limit"`

	VolumeRequest     VolumeBindings `json:"volume_request"`
	VolumeLimit       VolumeBindings `json:"volume_limit"`
	VolumePlanRequest VolumePlan     `json:"volume_plan_request"`
	VolumePlanLimit   VolumePlan     `json:"volume_plan_limit"`
	VolumeChanged     bool           `json:"volume_changed"`

	StorageRequest int64 `json:"storage_request"`
	StorageLimit   int64 `json:"storage_limit"`
}

// ResourceType .
type ResourceType int

const (
	// ResourceCPU .
	ResourceCPU ResourceType = 1 << iota
	// ResourceCPUBind .
	ResourceCPUBind
	// ResourceMemory .
	ResourceMemory
	// ResourceVolume .
	ResourceVolume
	// ResourceScheduledVolume .
	ResourceScheduledVolume
	// ResourceStorage .
	ResourceStorage
)

var (
	// ResourceAll .
	ResourceAll = ResourceStorage | ResourceMemory | ResourceCPU | ResourceVolume
	// AllResourceTypes .
	AllResourceTypes = [...]ResourceType{ResourceCPU, ResourceMemory, ResourceVolume, ResourceStorage}
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
	if *p == nil {
		*p = VolumePlan{}
	}
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
				// p[v] = VolumeMap{oldBinding.GetResourceID(): 0}
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
