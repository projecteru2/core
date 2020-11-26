package volume

import (
	"testing"

	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	schedulerMocks "github.com/projecteru2/core/scheduler/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMakeRequest(t *testing.T) {
	_, err := MakeRequest(types.ResourceOptions{
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
		},
	})
	assert.Nil(t, err)

	_, err = MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
			{
				Source:      "/data2",
				Destination: "/data2",
			},
		},
	})
	assert.NotNil(t, err)

}

func TestStorage(t *testing.T) {
	mockScheduler := &schedulerMocks.Scheduler{}
	var (
		volumePlans = []types.VolumePlan{
			{
				types.VolumeBinding{
					Source:      "/data1",
					Destination: "/data1",
				}: types.VolumeMap{
					"/data1": 512,
				},
			},
		}
		nodeInfos []types.NodeInfo = []types.NodeInfo{
			{
				Name:       "TestNode",
				CPU:     map[string]int64{"0": 10000, "1": 10000},
				NUMA:       map[string]string{"0": "0", "1": "1"},
				NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
				MemCap:     10240,
				CPUPlan:    []types.CPUMap{{"0": 10000, "1": 10000}},
				StorageCap: 10240,
				Volume: types.VolumeMap{
					"/data1": 1024,
					"/data2": 1024,
				},
				InitVolume: types.VolumeMap{
					"/data0": 1024,
				},
				VolumePlans: volumePlans,
			},
		}
		volumePlan = map[string][]types.VolumePlan{
			"TestNode": volumePlans,
		}
	)
	mockScheduler.On(
		"SelectVolumeNodes", mock.Anything, mock.Anything,
	).Return(nodeInfos, volumePlan, 1, nil)

	prevSche, _ := scheduler.GetSchedulerV1()
	scheduler.InitSchedulerV1(mockScheduler)
	defer func() {
		scheduler.InitSchedulerV1(prevSche)
	}()

	resourceRequest, err := MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
		},
	})
	assert.Nil(t, err)
	assert.True(t, resourceRequest.Type()&types.ResourceVolume > 0)

	sche := resourceRequest.MakeScheduler()

	plans, _, err := sche(nodeInfos)
	assert.Nil(t, err)
	assert.True(t, plans.Type()&types.ResourceVolume > 0)

	const storage = int64(10240)
	var node = types.Node{
		Name:       "TestNode",
		CPU:        map[string]int64{"0": 10000, "1": 10000},
		NUMA:       map[string]string{"0": "0", "1": "1"},
		NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
		MemCap:     10240,
		StorageCap: storage,
		Volume:     types.VolumeMap{"/data1": 512, "/data2": 512},
		VolumeUsed: 0,
		InitVolume: types.VolumeMap{"/data1": 512, "/data2": 512},
	}

	assert.NotNil(t, plans.Capacity())
	plans.ApplyChangesOnNode(&node, 0)
	assert.Less(t, node.Volume["/data1"], int64(512))

	plans.RollbackChangesOnNode(&node, 0)
	assert.Equal(t, node.Volume["/data1"], int64(512))

	opts := resourcetypes.DispenseOptions{
		Node:  &node,
		Index: 0,
		ExistingInstance: &types.Workload{
			ResourceMeta: types.ResourceMeta{
				VolumePlanRequest: types.VolumePlan{
					types.VolumeBinding{
						Source:      "/data1",
						Destination: "/data1",
					}: types.VolumeMap{
						"/data1": 512,
					},
				},
			},
		},
	}
	r := &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Nil(t, err)
}

func TestRate(t *testing.T) {
	req, err := MakeRequest(types.ResourceOptions{
		VolumeRequest: types.VolumeBindings{&types.VolumeBinding{SizeInBytes: 1024}},
		VolumeLimit:   types.VolumeBindings{&types.VolumeBinding{SizeInBytes: 1024}},
	})
	assert.Nil(t, err)
	node := types.Node{
		Volume: types.VolumeMap{"1": 1024},
	}
	assert.Equal(t, req.Rate(node), 1.0)
}
