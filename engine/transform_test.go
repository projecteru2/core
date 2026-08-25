package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	resourcetypes "github.com/projecteru2/core/resource/types"
)

func TestVirtualizationResourceDecode(t *testing.T) {
	engineParams := resourcetypes.Resources{
		"cpumem": {
			"cpu_map": map[string]int64{"1": 100},
			"cpu":     100.0,
			"memory":  10000,
		},
	}

	dst := &VirtualizationResource{}
	assert.NoError(t, dst.Decode(engineParams))
	assert.Equal(t, 100.0, dst.Quota)
	assert.Len(t, dst.CPU, 1)
	assert.EqualValues(t, 10000, dst.Memory)

	assert.NoError(t, (&VirtualizationResource{}).Decode(resourcetypes.Resources{}))
}

func TestVirtualizationResourceDecodeMergesEveryPlugin(t *testing.T) {
	engineParams := resourcetypes.Resources{
		"cpumem": {
			"cpu_map": map[string]int64{"1": 100},
			"cpu":     2.0,
			"memory":  10000,
		},
		"storage": {
			"storage":      20000,
			"volumes":      []string{"/data:/data"},
			"iops_options": map[string]string{"/dev/vda": "1000:1000:0:0"},
		},
	}

	dst := &VirtualizationResource{}
	assert.NoError(t, dst.Decode(engineParams))
	assert.Equal(t, 2.0, dst.Quota)
	assert.Equal(t, map[string]int64{"1": 100}, dst.CPU)
	assert.EqualValues(t, 10000, dst.Memory)
	assert.EqualValues(t, 20000, dst.Storage)
	assert.Equal(t, []string{"/data:/data"}, dst.Volumes)
	assert.Equal(t, map[string]string{"/dev/vda": "1000:1000:0:0"}, dst.IOPSOptions)
}
