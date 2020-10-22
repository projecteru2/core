package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockReq struct {
}

func (m *mockReq) Type() ResourceType {
	return ResourceCPU | ResourceCPUBind
}

func (m *mockReq) DeployValidate() error {
	return nil
}

func (m *mockReq) MakeScheduler() SchedulerV2 {
	return nil
}

func (m *mockReq) Rate(_ Node) float64 {
	return 0.24
}

type mockPlan struct {
}

func (m *mockPlan) Type() ResourceType {
	return ResourceCPU
}

func (m *mockPlan) Capacity() map[string]int {
	return map[string]int{
		"1": 1,
	}
}

func (m *mockPlan) ApplyChangesOnNode(_ *Node, _ ...int) {}

func (m *mockPlan) RollbackChangesOnNode(_ *Node, _ ...int) {}

func (m *mockPlan) Dispense(_ DispenseOptions, _ *Resources) error { return nil }

func TestStrategyInfos(t *testing.T) {
	opts := &DeployOptions{
		ResourceRequests: []ResourceRequest{&mockReq{}},
	}
	nodeMap := map[string]*Node{
		"1": {
			CPUUsed:        1.5,
			InitCPU:        CPUMap{"0": 200, "1": 200},
			VolumeUsed:     500,
			InitVolume:     VolumeMap{"/data1": 1000, "/data2": 2500},
			MemCap:         1,
			InitMemCap:     100,
			StorageCap:     0,
			InitStorageCap: 2,
		},
	}
	planMap := map[ResourceType]ResourcePlans{
		ResourceCPU: &mockPlan{},
	}
	sis := NewStrategyInfos(opts, nodeMap, planMap)
	assert.Equal(t, 1, len(sis))
	assert.EqualValues(t, 1.5/2., sis[0].GetResourceUsage(ResourceCPU))
	assert.EqualValues(t, 1.5/2.+.99, sis[0].GetResourceUsage(ResourceCPU|ResourceCPUBind|ResourceMemory))
	assert.EqualValues(t, 0.24, sis[0].GetResourceRate(ResourceCPU))
}
