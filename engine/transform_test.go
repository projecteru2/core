package engine

import (
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/stretchr/testify/assert"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

func TestMakeVirtualizationResource(t *testing.T) {
	engineParams := resourcetypes.Resources{
		"cpumem": {
			"cpu_map": map[string]int64{"1": 100},
			"cpu":     100.0,
			"memory":  10000,
		},
	}

	dst := &virtualizationResource{}

	err := MakeVirtualizationResource(engineParams, dst, func(p resourcetypes.Resources, d *virtualizationResource) error {
		return mapstructure.Decode(p["cpumem"], d)
	})
	assert.NoError(t, err)
	assert.Equal(t, dst.Quota, 100.0)
	assert.Len(t, dst.CPU, 1)
	err = MakeVirtualizationResource(engineParams, dst, func(p resourcetypes.Resources, d *virtualizationResource) error {
		return mapstructure.Decode(p["storage"], d)
	})
	assert.NoError(t, err)
}

type virtualizationResource struct {
	CPU           map[string]int64            `json:"cpu_map" mapstructure:"cpu_map"`
	Quota         float64                     `json:"cpu" mapstructure:"cpu"`
	Memory        int64                       `json:"memory" mapstructure:"memory"`
	Storage       int64                       `json:"storage" mapstructure:"storage"`
	NUMANode      string                      `json:"numa_node" mapstructure:"numa_node"`
	Volumes       []string                    `json:"volumes" mapstructure:"volumes"`
	VolumePlan    map[string]map[string]int64 `json:"volume_plan" mapstructure:"volume_plan"`
	VolumeChanged bool                        `json:"volume_changed" mapstructure:"volume_changed"`
	IOPSOptions   map[string]string           `json:"iops_options" mapstructure:"IOPS_options"`
	Remap         bool                        `json:"remap" mapstructure:"remap"`
}
