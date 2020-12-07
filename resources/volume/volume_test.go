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

	// Source not match
	_, err = MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data2",
				Destination: "/data1",
			},
		},
	})
	assert.NotNil(t, err)

	// Dest not match
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
				Destination: "/data2",
			},
		},
	})
	assert.NotNil(t, err)

	// Flag not match
	_, err = MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
				Flags:       "r",
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
				Flags:       "rw",
			},
		},
	})
	assert.NotNil(t, err)

	// Request SizeInBytes larger then limit
	_, err = MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
				SizeInBytes: 10240,
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
				SizeInBytes: 5120,
			},
		},
	})
	assert.NoError(t, err)

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

	_, err = MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
			{
				Source:      "/data3",
				Destination: "/data3",
			},
			{
				Source:      "/data2",
				Destination: "/data2",
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
			},
			{
				Source:      "/data4",
				Destination: "/data4",
			},
		},
	})
	assert.NotNil(t, err)

}

func TestType(t *testing.T) {
	resourceRequest, err := MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "AUTO",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "AUTO",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
	})
	assert.Nil(t, err)
	assert.True(t, resourceRequest.Type()&(types.ResourceVolume|types.ResourceScheduledVolume) > 0)
}

func TestStoragePlans(t *testing.T) {
	testStoragePlans(t, types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "AUTO",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "AUTO",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 256,
			},
		},
	})
	testStoragePlans(t, types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "AUTO",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "AUTO",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
	})
}

func testStoragePlans(t *testing.T, reqOpts types.ResourceOptions) {
	mockScheduler := &schedulerMocks.Scheduler{}
	var (
		volumePlans = []types.VolumePlan{
			{
				types.VolumeBinding{
					Source:      "AUTO",
					Destination: "/data1",
					Flags:       "rw",
					SizeInBytes: 128,
				}: types.VolumeMap{
					"/dev0": 512,
				},
			},
			{
				types.VolumeBinding{
					Source:      "AUTO",
					Destination: "/data1",
					Flags:       "rw",
					SizeInBytes: 128,
				}: types.VolumeMap{
					"/dev1": 512,
				},
			},
		}
		scheduleInfos []resourcetypes.ScheduleInfo = []resourcetypes.ScheduleInfo{
			{
				NodeMeta: types.NodeMeta{
					Name:       "TestNode",
					CPU:        map[string]int64{"0": 10000, "1": 10000},
					NUMA:       map[string]string{"0": "0", "1": "1"},
					NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
					MemCap:     10240,
					StorageCap: 10240,
					Volume: types.VolumeMap{
						"/data1": 1024,
						"/data2": 1024,
					},
					InitVolume: types.VolumeMap{
						"/data0": 1024,
					},
				},
				VolumePlans: volumePlans,
				CPUPlan:     []types.CPUMap{{"0": 10000, "1": 10000}},
				Capacity:    100,
			},
		}
		volumePlan = map[string][]types.VolumePlan{
			"TestNode": volumePlans,
		}
	)

	resourceRequest, err := MakeRequest(reqOpts)
	assert.Nil(t, err)
	assert.True(t, resourceRequest.Type()&types.ResourceVolume > 0)
	sche := resourceRequest.MakeScheduler()

	mockScheduler.On(
		"SelectVolumeNodes", mock.Anything, mock.Anything,
	).Return(scheduleInfos, volumePlan, 1, nil)

	prevSche, _ := scheduler.GetSchedulerV1()
	scheduler.InitSchedulerV1(nil)

	plans, _, err := sche(scheduleInfos)
	assert.Error(t, err)

	scheduler.InitSchedulerV1(mockScheduler)
	defer func() {
		scheduler.InitSchedulerV1(prevSche)
	}()

	plans, _, err = sche(scheduleInfos)
	assert.Nil(t, err)
	assert.True(t, plans.Type()&types.ResourceVolume > 0)

	const storage = int64(10240)
	var node = types.Node{
		NodeMeta: types.NodeMeta{
			Name:       "TestNode",
			CPU:        map[string]int64{"0": 10000, "1": 10000},
			NUMA:       map[string]string{"0": "0", "1": "1"},
			NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
			MemCap:     10240,
			StorageCap: storage,
			Volume:     types.VolumeMap{"/dev0": 10240, "/dev1": 5120},
		},
	}

	assert.NotNil(t, plans.Capacity())
	plans.ApplyChangesOnNode(&node, 0, 1)
	assert.Less(t, node.Volume["/dev0"], int64(10240))
	assert.Less(t, node.Volume["/dev1"], int64(5120))

	plans.RollbackChangesOnNode(&node, 0, 1)
	assert.Equal(t, node.Volume["/dev0"], int64(10240))
	assert.Equal(t, node.Volume["/dev1"], int64(5120))

	opts := resourcetypes.DispenseOptions{
		Node:  &node,
		Index: 0,
	}
	r := &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Nil(t, err)

	if reqOpts.VolumeRequest[0].SizeInBytes != reqOpts.VolumeLimit[0].SizeInBytes {
		diff := reqOpts.VolumeLimit[0].SizeInBytes - reqOpts.VolumeRequest[0].SizeInBytes
		assert.Equal(t, int64(512)+diff, r.VolumePlanLimit[*reqOpts.VolumeLimit[0]]["/dev0"])
		return
	}
	assert.Equal(t, int64(512), r.VolumePlanLimit[*reqOpts.VolumeRequest[0]]["/dev0"])
}

func TestStorage(t *testing.T) {
	mockScheduler := &schedulerMocks.Scheduler{}
	var (
		volumePlans = []types.VolumePlan{
			{
				types.VolumeBinding{
					Source:      "/data1",
					Destination: "/data1",
					Flags:       "rw",
					SizeInBytes: 128,
				}: types.VolumeMap{
					"/dev0": 512,
				},
			},
		}
		scheduleInfos []resourcetypes.ScheduleInfo = []resourcetypes.ScheduleInfo{
			{
				NodeMeta: types.NodeMeta{
					Name: "TestNode",
				},
				Capacity: 100,
			},
		}
		volumePlan = map[string][]types.VolumePlan{
			"TestNode": volumePlans,
		}
	)

	resourceRequest, err := MakeRequest(types.ResourceOptions{
		VolumeRequest: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
		VolumeLimit: []*types.VolumeBinding{
			{
				Source:      "/data1",
				Destination: "/data1",
				Flags:       "rw",
				SizeInBytes: 128,
			},
		},
	})
	assert.Nil(t, err)
	assert.True(t, resourceRequest.Type()&types.ResourceVolume > 0)
	sche := resourceRequest.MakeScheduler()

	mockScheduler.On(
		"SelectVolumeNodes", mock.Anything, mock.Anything,
	).Return(scheduleInfos, volumePlan, 1, nil)

	prevSche, _ := scheduler.GetSchedulerV1()
	scheduler.InitSchedulerV1(nil)

	plans, _, err := sche(scheduleInfos)
	assert.Error(t, err)

	scheduler.InitSchedulerV1(mockScheduler)
	defer func() {
		scheduler.InitSchedulerV1(prevSche)
	}()

	plans, _, err = sche(scheduleInfos)
	assert.Nil(t, err)
	assert.True(t, plans.Type()&types.ResourceVolume > 0)

	const storage = int64(10240)
	var node = types.Node{
		NodeMeta: types.NodeMeta{
			Name:       "TestNode",
			CPU:        map[string]int64{"0": 10000, "1": 10000},
			NUMA:       map[string]string{"0": "0", "1": "1"},
			NUMAMemory: map[string]int64{"0": 1024, "1": 1204},
			MemCap:     10240,
			StorageCap: storage,
			Volume:     types.VolumeMap{"/dev0": 10240, "/dev1": 5120},
		},
		VolumeUsed: 0,
	}

	assert.NotNil(t, plans.Capacity())
	plans.ApplyChangesOnNode(&node, 0)
	assert.Less(t, node.Volume["/dev0"], int64(10240))
	assert.Equal(t, node.Volume["/dev1"], int64(5120))

	plans.RollbackChangesOnNode(&node, 0)
	assert.Equal(t, node.Volume["/dev0"], int64(10240))
	assert.Equal(t, node.Volume["/dev1"], int64(5120))

	opts := resourcetypes.DispenseOptions{
		Node:  &node,
		Index: 0,
	}
	r := &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Nil(t, err)

	opts = resourcetypes.DispenseOptions{
		Node:  &node,
		Index: 0,
		ExistingInstance: &types.Workload{
			ResourceMeta: types.ResourceMeta{
				VolumePlanRequest: types.VolumePlan{
					types.VolumeBinding{
						Source:      "/data1",
						Destination: "/data1",
					}: types.VolumeMap{
						"/dev0": 5120,
					},
				},
			},
		},
	}
	r = &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Nil(t, err)

	opts = resourcetypes.DispenseOptions{
		Node:  &node,
		Index: 0,
		ExistingInstance: &types.Workload{
			ResourceMeta: types.ResourceMeta{
				VolumePlanRequest: types.VolumePlan{
					types.VolumeBinding{
						Source:      "/data1",
						Destination: "/data1",
					}: types.VolumeMap{
						"/data3": 512,
					},
				},
			},
		},
	}
	r = &types.ResourceMeta{}
	_, err = plans.Dispense(opts, r)
	assert.Error(t, err)
}

func TestRate(t *testing.T) {
	req, err := MakeRequest(types.ResourceOptions{
		VolumeRequest: types.VolumeBindings{&types.VolumeBinding{SizeInBytes: 1024}},
		VolumeLimit:   types.VolumeBindings{&types.VolumeBinding{SizeInBytes: 1024}},
	})
	assert.Nil(t, err)
	node := types.Node{
		NodeMeta: types.NodeMeta{
			Volume: types.VolumeMap{"1": 1024},
		},
	}
	assert.Equal(t, req.Rate(node), 1.0)
}
