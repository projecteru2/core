package complexscheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestGlobalDivisionPlan1(t *testing.T) {
	n1 := types.NodeInfo{
		Name:     "n1",
		CPUUsage: 0.1,
		MemUsage: 0.7,
		CPURate:  0.03,
		MemRate:  0.02,
		Capacity: 1,
	}
	n2 := types.NodeInfo{
		Name:     "n2",
		CPUUsage: 0.2,
		MemUsage: 0.3,
		CPURate:  0.04,
		MemRate:  0.07,
		Capacity: 1,
	}
	n3 := types.NodeInfo{
		Name:     "n3",
		CPUUsage: 1.3,
		MemUsage: 0.9,
		CPURate:  0.01,
		MemRate:  0.04,
		Capacity: 1,
	}
	arg := []types.NodeInfo{n3, n2, n1}
	r, err := GlobalDivisionPlan(arg, 3)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 1)
}

func TestGlobalDivisionPlan2(t *testing.T) {
	n1 := types.NodeInfo{
		Name:     "n1",
		CPUUsage: 0.9,
		MemUsage: 0.7,
		CPURate:  0.03,
		MemRate:  0.02,
		Capacity: 100,
	}
	n2 := types.NodeInfo{
		Name:     "n2",
		CPUUsage: 0.2,
		MemUsage: 0.3,
		CPURate:  0.04,
		MemRate:  0.07,
		Capacity: 100,
	}
	arg := []types.NodeInfo{n2, n1}
	r, err := GlobalDivisionPlan(arg, 2)
	assert.NoError(t, err)
	assert.Equal(t, r[0].Deploy, 2)
}
