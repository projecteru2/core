package types

import (
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

type mockResourceRequest struct {
	t    types.ResourceType
	rate float64
}

func (m *mockResourceRequest) Type() types.ResourceType { return m.t }

func (m *mockResourceRequest) Validate() error { return nil }

func (m *mockResourceRequest) MakeScheduler() SchedulerV2 { return nil }

func (m *mockResourceRequest) Rate(node types.Node) float64 { return m.rate }

func TestResourceRequestsMethod(t *testing.T) {
	node := types.Node{
		NodeMeta: types.NodeMeta{
			InitCPU:        types.CPUMap{"0": 100},
			InitMemCap:     100,
			InitStorageCap: 200,
			InitVolume:     types.VolumeMap{"/sda": 200},
		},
		CPUUsed: 0.5,
	}
	rrs := ResourceRequests{
		&mockResourceRequest{t: types.ResourceCPUBind},
		&mockResourceRequest{t: types.ResourceStorage},
		&mockResourceRequest{t: types.ResourceScheduledVolume},
	}
	assert.EqualValues(t, types.ResourceCPU|types.ResourceVolume, rrs.MainResourceType())
	assert.EqualValues(t, 0, rrs.MainRateOnNode(node))
	assert.EqualValues(t, 0.5, rrs.MainUsageOnNode(node))

	rrs = ResourceRequests{
		&mockResourceRequest{t: types.ResourceMemory},
		&mockResourceRequest{t: types.ResourceStorage},
		&mockResourceRequest{t: types.ResourceVolume},
	}
	assert.EqualValues(t, types.ResourceMemory, rrs.MainResourceType())
	assert.EqualValues(t, 0, rrs.MainRateOnNode(node))
	assert.EqualValues(t, 1, rrs.MainUsageOnNode(node))

}
