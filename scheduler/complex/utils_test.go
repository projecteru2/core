package complexscheduler

import (
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestScoreSort(t *testing.T) {
	ns := []types.NodeInfo{
		{
			Name: "n1",
			Usages: map[types.ResourceType]float64{
				types.ResourceCPU:    0.1,
				types.ResourceVolume: 0.3,
				types.ResourceMemory: 0.4,
			},
		},
		{
			Name: "n2",
			Usages: map[types.ResourceType]float64{
				types.ResourceCPU:    0.3,
				types.ResourceVolume: 0.3,
				types.ResourceMemory: 0.1,
			},
		},
		{
			Name: "n3",
			Usages: map[types.ResourceType]float64{
				types.ResourceCPU:    0.2,
				types.ResourceVolume: 0.3,
				types.ResourceMemory: 0.1,
			},
		},
	}

	scoreSort(ns, types.ResourceCPU)
	assert.Equal(t, ns[0].Name, "n1")
	assert.Equal(t, ns[1].Name, "n3")
	assert.Equal(t, ns[2].Name, "n2")

	scoreSort(ns, types.ResourceCPU|types.ResourceMemory)
	assert.Equal(t, ns[0].Name, "n3")
	assert.Equal(t, ns[1].Name, "n2")
	assert.Equal(t, ns[2].Name, "n1")
}
