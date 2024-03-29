package engine

import (
	resourcetypes "github.com/projecteru2/core/resource/types"
)

// TODO 可以考虑进一步简化，每个 engine 自行处理

// VirtualizationResource define resources
type VirtualizationResource struct {
	CPU           map[string]int64            `json:"cpu_map" mapstructure:"cpu_map"` // for cpu binding
	Quota         float64                     `json:"cpu" mapstructure:"cpu"`         // for cpu quota
	Memory        int64                       `json:"memory" mapstructure:"memory"`   // for memory binding
	Storage       int64                       `json:"storage" mapstructure:"storage"`
	NUMANode      string                      `json:"numa_node" mapstructure:"numa_node"` // numa node
	Volumes       []string                    `json:"volumes" mapstructure:"volumes"`
	VolumePlan    map[string]map[string]int64 `json:"volume_plan" mapstructure:"volume_plan"`       // literal VolumePlan
	VolumeChanged bool                        `json:"volume_changed" mapstructure:"volume_changed"` // indicate whether new volumes contained in realloc request
	IOPSOptions   map[string]string           `json:"iops_options" mapstructure:"IOPS_options"`     // format: {device_name: "read-IOPS:write-IOPS:read-bps:write-bps"}
	Remap         bool                        `json:"remap" mapstructure:"remap"`
}

// MakeVirtualizationResource .
func MakeVirtualizationResource[T any](engineParams resourcetypes.Resources, dst T, f func(resourcetypes.Resources, T) error) error {
	return f(engineParams, dst)
}
