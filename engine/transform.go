package engine

import (
	"github.com/go-viper/mapstructure/v2"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

// VirtualizationResource is the decoded per-engine resource view of a workload.
type VirtualizationResource struct {
	CPU           map[string]int64            `json:"cpu_map" mapstructure:"cpu_map"` // cpu id to share
	Quota         float64                     `json:"cpu" mapstructure:"cpu"`
	Memory        int64                       `json:"memory" mapstructure:"memory"`
	Storage       int64                       `json:"storage" mapstructure:"storage"`
	NUMANode      string                      `json:"numa_node" mapstructure:"numa_node"`
	Volumes       []string                    `json:"volumes" mapstructure:"volumes"`
	VolumePlan    map[string]map[string]int64 `json:"volume_plan" mapstructure:"volume_plan"`
	VolumeChanged bool                        `json:"volume_changed" mapstructure:"volume_changed"` // set when a realloc request brings new volumes
	IOPSOptions   map[string]string           `json:"iops_options" mapstructure:"IOPS_options"`     // format: {device_name: "read-IOPS:write-IOPS:read-bps:write-bps"}
	Remap         bool                        `json:"remap" mapstructure:"remap"`
}

// Decode merges every plugin's engine params into r.
func (r *VirtualizationResource) Decode(engineParams resourcetypes.Resources) error {
	for _, params := range engineParams {
		if err := mapstructure.Decode(params, r); err != nil {
			return err
		}
	}
	return nil
}
