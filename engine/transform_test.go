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
