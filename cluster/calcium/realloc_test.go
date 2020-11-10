package calcium

import (
	"github.com/projecteru2/core/types"
)

func newReallocOptions(id string, cpu float64, memory int64, vbs types.VolumeBindings, bindCPUOpt, memoryLimitOpt types.TriOptions) *types.ReallocOptions {
	return &types.ReallocOptions{
		ID:          id,
		CPUBindOpts: bindCPUOpt,
		ResourceOpts: types.ResourceOptions{
			CPUQuotaLimit: cpu,
			MemoryLimit:   memory,
			VolumeLimit:   vbs,
		},
	}
}

/*
func TestRealloc(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	c.config.Scheduler.ShareBase = 100

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	pod1 := &types.Pod{
		Name: "p1",
	}

	node1 := &types.Node{
		Name:       "node1",
		MemCap:     int64(units.GiB),
		CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		InitCPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
		Engine:     engine,
		Endpoint:   "http://1.1.1.1:1",
		NUMA:       types.NUMA{"2": "0"},
		NUMAMemory: types.NUMAMemory{"0": 100000},
		Volume:     types.VolumeMap{"/dir0": 100},
	}

	c1 := &types.Container{
		ID:      "c1",
		Podname: "p1",
		Engine:  engine,
		ResourceMeta: types.ResourceMeta{
			MemoryLimit:       5 * int64(units.MiB),
			MemoryRequest:     5 * int64(units.MiB),
			CPUQuotaLimit:     0.9,
			CPUQuotaRequest:   0.9,
			CPU:               types.CPUMap{"2": 90},
			VolumePlanRequest: types.VolumePlan{types.MustToVolumeBinding("AUTO:/data:rw:50"): types.VolumeMap{"/dir0": 50}},
			VolumeRequest:     types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"}),
			VolumePlanLimit:   types.VolumePlan{types.MustToVolumeBinding("AUTO:/data:rw:50"): types.VolumeMap{"/dir0": 50}},
			VolumeLimit:       types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"}),
		},
		Nodename: "node1",
	}

	c2 := &types.Container{
		ID:      "c2",
		Podname: "p1",
		Engine:  engine,
		ResourceMeta: types.ResourceMeta{
			MemoryRequest:   5 * int64(units.MiB),
			MemoryLimit:     5 * int64(units.MiB),
			CPUQuotaLimit:   0.9,
			CPUQuotaRequest: 0.9,
		},
		Nodename: "node1",
	}

	store.On("GetContainers", mock.Anything, []string{"c1"}).Return([]*types.Container{c1}, nil)
	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err := c.ReallocResource(ctx, newReallocOptions("c1", -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	store.AssertExpectations(t)

	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by GetPod
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, types.ErrNoETCD).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	store.AssertExpectations(t)

	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	// failed by newCPU < 0
	err = c.ReallocResource(ctx, newReallocOptions("c1", -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	store.AssertExpectations(t)

	// failed by GetNode
	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrNoETCD).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	store.AssertExpectations(t)

	// failed by no new CPU Plan
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	simpleMockScheduler := &schedulermocks.Scheduler{}
	scheduler.InitSchedulerV1(simpleMockScheduler)
	c.scheduler = simpleMockScheduler
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientMEM).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	store.AssertExpectations(t)
	simpleMockScheduler.AssertExpectations(t)

	// failed by wrong total
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, nil).Once()
	simpleMockScheduler.On("SelectStorageNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 100, nil)
	nodeVolumePlans := map[string][]types.VolumePlan{
		"node1": {{types.MustToVolumeBinding("AUTO:/data:rw:50"): types.VolumeMap{"/dir0": 50}}},
	}
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"})).Return(nil, nodeVolumePlans, 1, nil)
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// vaild cpu plans
	nodeCPUPlans := map[string][]types.CPUMap{
		node1.Name: {
			{"3": 100},
			{"2": 100},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.VolumeBindings{}).Return(nil, nil, 100, nil)
	// failed by apply resource
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Twice()
	// update node failed
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Times(4)
	// reset node
	node1 = &types.Node{
		Name:     "node1",
		MemCap:   int64(units.GiB),
		CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Engine:   engine,
		Endpoint: "http://1.1.1.1:1",
	}
	store.On("GetContainers", mock.Anything, "c2").Return([]*types.Container{c2}, nil)
	err = c.ReallocResource(ctx, newReallocOptions("c2", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)

	// check node resource as usual
	assert.Equal(t, node1.CPU["2"], int64(10))
	assert.Equal(t, node1.MemCap, int64(units.GiB))
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 2, nil).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)

	// failed by update container
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Times(4)
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// failed by volume binding incompatible
	nodeVolumePlans = map[string][]types.VolumePlan{
		node1.Name: {
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir1": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir2": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir3": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir4": 100}},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data:rw:100"})).Return(nil, nodeVolumePlans, 4, nil).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)

	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// failed by volume schedule error
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientVolume).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:1"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// failed due to re-volume plan less then container number
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nodeVolumePlans, 0, nil).Once()
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:1"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	simpleMockScheduler.AssertExpectations(t)

	// good to go
	// rest everything
	node2 := &types.Node{
		Name:       "node2",
		MemCap:     int64(units.GiB),
		CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		InitCPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
		Engine:     engine,
		Endpoint:   "http://1.1.1.1:1",
		NUMA:       types.NUMA{"2": "0"},
		NUMAMemory: types.NUMAMemory{"0": 100000},
		Volume:     types.VolumeMap{"/dir0": 200, "/dir1": 200, "/dir2": 200},
		VolumeUsed: int64(300),
	}
	c3 := &types.Container{
		ID:      "c3",
		Podname: "p1",
		Engine:  engine,
		ResourceMeta: types.ResourceMeta{
			MemoryLimit:     5 * int64(units.MiB),
			MemoryRequest:   5 * int64(units.MiB),
			CPUQuotaLimit:   0.9,
			CPUQuotaRequest: 0.9,
			CPU:             types.CPUMap{"2": 90},
			VolumeRequest:   types.MustToVolumeBindings([]string{"AUTO:/data0:rw:100", "AUTO:/data1:rw:200"}),
			VolumePlanRequest: types.VolumePlan{
				types.MustToVolumeBinding("AUTO:/data0:rw:100"): types.VolumeMap{"/dir0": 100},
				types.MustToVolumeBinding("AUTO:/data1:rw:200"): types.VolumeMap{"/dir1": 200},
			},
			VolumeLimit: types.MustToVolumeBindings([]string{"AUTO:/data0:rw:100", "AUTO:/data1:rw:200"}),
			VolumePlanLimit: types.VolumePlan{
				types.MustToVolumeBinding("AUTO:/data0:rw:100"): types.VolumeMap{"/dir0": 100},
				types.MustToVolumeBinding("AUTO:/data1:rw:200"): types.VolumeMap{"/dir1": 200},
			},
		},
		Nodename: "node2",
	}
	nodeCPUPlans = map[string][]types.CPUMap{
		node2.Name: {
			{"3": 100},
			{"2": 100},
		},
	}
	nodeVolumePlans = map[string][]types.VolumePlan{
		node2.Name: {
			{
				types.MustToVolumeBinding("AUTO:/data0:rw:50"):  types.VolumeMap{"/dir1": 50},
				types.MustToVolumeBinding("AUTO:/data1:rw:200"): types.VolumeMap{"/dir2": 200},
			},
			{
				types.MustToVolumeBinding("AUTO:/data0:rw:50"):  types.VolumeMap{"/dir0": 50},
				types.MustToVolumeBinding("AUTO:/data1:rw:200"): types.VolumeMap{"/dir1": 200},
			},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil)
	simpleMockScheduler.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data0:rw:50", "AUTO:/data1:rw:200"})).Return(nil, nodeVolumePlans, 2, nil)
	store.On("GetNode", mock.Anything, "node2").Return(node2, nil)
	store.On("GetContainers", mock.Anything, "c3").Return([]*types.Container{c3}, nil)
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Times(4)
	err = c.ReallocResource(ctx, newReallocOptions("c3", 0.1, 2*int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data0:rw:-50"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	assert.Equal(t, node2.CPU["3"], int64(100))
	assert.Equal(t, node2.CPU["2"], int64(10))
	assert.Equal(t, node2.MemCap, int64(units.GiB))
	assert.Equal(t, node2.Volume, types.VolumeMap{"/dir0": 200, "/dir1": 200, "/dir2": 200})
	assert.Equal(t, node2.VolumeUsed, int64(300))
	store.AssertExpectations(t)
	simpleMockScheduler.AssertExpectations(t)
}
*/

/*
func TestReallocVolume(t *testing.T) {
	c := NewTestCluster()
	store := &storemocks.Store{}
	c.store = store

	simpleMockScheduler := &schedulermocks.Scheduler{}
	c.scheduler = simpleMockScheduler
	scheduler.InitSchedulerV1(simpleMockScheduler)
	engine := &enginemocks.API{}

	node1 := &types.Node{
		Name:       "node1",
		Volume:     types.VolumeMap{"/data": 1000, "/data1": 1000, "/data2": 1000, "/data3": 1000},
		InitVolume: types.VolumeMap{"/data": 2000, "/data1": 2000, "/data2": 2000, "/data3": 2000},
		Engine:     engine,
	}

	c1 := &types.Container{
		ID:       "c1",
		Engine:   engine,
		Podname:  "p1",
		Nodename: "node1",
		ResourceMeta: types.ResourceMeta{
			VolumeRequest: types.MustToVolumeBindings([]string{"AUTO:/data:rw:0", "AUTO:/data1:rw:100", "AUTO:/data2:rw:0", "AUTO:/data3:rw:600"}),
			VolumeLimit:   types.MustToVolumeBindings([]string{"AUTO:/data:rw:0", "AUTO:/data1:rw:100", "AUTO:/data2:rw:0", "AUTO:/data3:rw:600"}),
			VolumePlanRequest: types.VolumePlan{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir0": 100},
				types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir0": 0},
				types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
			},
			VolumePlanLimit: types.VolumePlan{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir0": 100},
				types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir0": 0},
				types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
			},
		},
	}

	pod1 := &types.Pod{Name: "p1"}

	newVbs := types.MustToVolumeBindings([]string{
		"AUTO:/data:rw:0",
		"AUTO:/data1:rw:-100",
		"AUTO:/data2:ro:110",
		"AUTO:/data3:rw:-580",
	})

	// test 1: incompatible

	newPlans := map[string][]types.VolumePlan{
		"node1": {
			{
				*newVbs[0]: types.VolumeMap{"/dir1": 0},
				*newVbs[1]: types.VolumeMap{"/dir1": 0},
				*newVbs[2]: types.VolumeMap{"/dir0": 110},
				*newVbs[3]: types.VolumeMap{"/dir1": 20},
			},
		},
	}

	ctx := context.Background()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, newPlans, 1, nil).Once()
	simpleMockScheduler.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 100, nil)
	simpleMockScheduler.On("SelectStorageNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 100, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	store.On("GetContainers", mock.Anything, []string{"c1"}).Return([]*types.Container{c1}, nil)
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	err := c.ReallocResource(ctx, newReallocOptions("c1", 0, 0, newVbs, types.TriKeep, types.TriKeep))
	assert.Nil(t, err)
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// test 2: modify unlimited volume map for compatible requirement

	newPlans = map[string][]types.VolumePlan{
		"node1": {
			{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:0"):   types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data2:rw:110"): types.VolumeMap{"/dir1": 110},
				types.MustToVolumeBinding("AUTO:/data3:rw:20"):  types.VolumeMap{"/dir1": 20},
			},
			{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:0"):   types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data2:rw:110"): types.VolumeMap{"/dir0": 110},
				types.MustToVolumeBinding("AUTO:/data3:rw:20"):  types.VolumeMap{"/dir0": 20},
			},
		},
	}

	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, newPlans, 1, nil).Once()
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0, 0, newVbs, types.TriKeep, types.TriKeep))
	assert.Nil(t, err)
	assert.EqualValues(t, 0, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data:rw:0")]["/dir0"])
	assert.EqualValues(t, 0, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data1:rw:0")]["/dir0"])
	assert.EqualValues(t, 110, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data2:rw:110")]["/dir0"])
	assert.EqualValues(t, 20, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data3:rw:20")]["/dir0"])
	simpleMockScheduler.AssertExpectations(t)

	// test 3: multiple containers search compatible respective plans

	newPlans = map[string][]types.VolumePlan{
		"node1": {
			{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:0"):   types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data2:rw:110"): types.VolumeMap{"/dir0": 110},
				types.MustToVolumeBinding("AUTO:/data3:rw:20"):  types.VolumeMap{"/dir1": 20},
			},
			{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:0"):   types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data2:rw:110"): types.VolumeMap{"/dir0": 110},
				types.MustToVolumeBinding("AUTO:/data3:rw:20"):  types.VolumeMap{"/dir0": 20},
			},
			{
				types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data1:rw:0"):   types.VolumeMap{"/dir1": 0},
				types.MustToVolumeBinding("AUTO:/data2:rw:110"): types.VolumeMap{"/dir1": 110},
				types.MustToVolumeBinding("AUTO:/data3:rw:20"):  types.VolumeMap{"/dir0": 20},
			},
		},
	}

	c1.VolumeRequest = types.MustToVolumeBindings([]string{"AUTO:/data:rw:0", "AUTO:/data1:rw:100", "AUTO:/data2:rw:0", "AUTO:/data3:rw:600"})
	c1.VolumeLimit = types.MustToVolumeBindings([]string{"AUTO:/data:rw:0", "AUTO:/data1:rw:100", "AUTO:/data2:rw:0", "AUTO:/data3:rw:600"})
	c1.VolumePlanLimit = types.VolumePlan{
		types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
		types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir0": 100},
		types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir0": 0},
		types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
	}
	c1.VolumePlanRequest = types.VolumePlan{
		types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
		types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir0": 100},
		types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir0": 0},
		types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
	}

	c2 := &types.Container{
		ID:            "c2",
		Engine:        engine,
		Podname:       "p1",
		Nodename:      "node1",
		VolumeRequest: types.MustToVolumeBindings([]string{"AUTO:/data:rw:0", "AUTO:/data1:rw:100", "AUTO:/data2:rw:0", "AUTO:/data3:rw:600"}),
		VolumeLimit:   types.MustToVolumeBindings([]string{"AUTO:/data:rw:0", "AUTO:/data1:rw:100", "AUTO:/data2:rw:0", "AUTO:/data3:rw:600"}),
		VolumePlanRequest: types.VolumePlan{
			types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
			types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir1": 100},
			types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir1": 0},
			types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
		},
		VolumePlanLimit: types.VolumePlan{
			types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
			types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir1": 100},
			types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir1": 0},
			types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
		},
	}

	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, newPlans, 3, nil).Once()

	err = c.ReallocResource(ctx, newReallocOptions("c1", 0, 0, newVbs, types.TriKeep, types.TriKeep))
	assert.Nil(t, err)
	assert.EqualValues(t, 0, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data:rw:0")]["/dir0"])
	assert.EqualValues(t, 0, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data1:rw:0")]["/dir0"])
	assert.EqualValues(t, 110, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data2:rw:110")]["/dir0"])
	assert.EqualValues(t, 20, c1.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data3:rw:20")]["/dir0"])
	assert.EqualValues(t, 0, c2.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data:rw:0")]["/dir0"])
	assert.EqualValues(t, 0, c2.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data1:rw:0")]["/dir1"])
	assert.EqualValues(t, 110, c2.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data2:rw:110")]["/dir1"])
	assert.EqualValues(t, 20, c2.VolumePlanRequest[types.MustToVolumeBinding("AUTO:/data3:rw:20")]["/dir0"])
}
*/

/*
func TestReallocBindCpu(t *testing.T) {
	c := NewTestCluster()
	c.config.Scheduler.ShareBase = 100
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	pod1 := &types.Pod{
		Name: "p1",
	}
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	config := types.Config{
		LogLevel:      "",
		Bind:          "",
		LockTimeout:   0,
		GlobalTimeout: 0,
		Statsd:        "",
		Profile:       "",
		CertPath:      "",
		Auth:          types.AuthConfig{},
		GRPCConfig:    types.GRPCConfig{},
		Git:           types.GitConfig{},
		Etcd:          types.EtcdConfig{},
		Docker:        types.DockerConfig{},
		Scheduler:     types.SchedConfig{MaxShare: -1, ShareBase: 100},
		Virt:          types.VirtConfig{},
		Systemd:       types.SystemdConfig{},
	}
	simpleMockScheduler, _ := complexscheduler.New(config)
	c.scheduler = simpleMockScheduler
	scheduler.InitSchedulerV1(simpleMockScheduler)

	//test bindCpu
	node3 := &types.Node{
		Name:       "node3",
		MemCap:     int64(units.GiB),
		CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		InitCPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
		CPUUsed:    2.1,
		Engine:     engine,
		Endpoint:   "http://1.1.1.1:1",
		NUMA:       types.NUMA{"2": "0"},
		NUMAMemory: types.NUMAMemory{"0": 100000},
		Volume:     types.VolumeMap{"/dir0": 200, "/dir1": 200, "/dir2": 200},
		VolumeUsed: int64(300),
	}
	c5 := &types.Container{
		ID:      "c5",
		Podname: "p1",
		Engine:  engine,
		ResourceMeta: types.ResourceMeta{
			MemoryRequest:   5 * int64(units.MiB),
			MemoryLimit:     5 * int64(units.MiB),
			CPUQuotaRequest: 0.9,
			CPUQuotaLimit:   0.9,
			CPU:             types.CPUMap{"2": 90},
		},
		Nodename: "node3",
	}
	c6 := &types.Container{
		ID:      "c6",
		Podname: "p1",
		Engine:  engine,
		ResourceMeta: types.ResourceMeta{
			MemoryRequest:   5 * int64(units.MiB),
			MemoryLimit:     5 * int64(units.MiB),
			CPUQuotaRequest: 0.9,
			CPUQuotaLimit:   0.9,
		},
		Nodename: "node3",
	}

	store.On("GetNode", mock.Anything, "node3").Return(node3, nil)
	store.On("GetContainers", mock.Anything, []string{"c5"}).Return([]*types.Container{c5}, nil)
	store.On("GetContainers", mock.Anything, []string{"c6"}).Return([]*types.Container{c6}, nil)
	store.On("GetContainers", mock.Anything, []string{"c6", "c5"}).Return([]*types.Container{c5, c6}, nil)
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil)
	err := c.ReallocResource(ctx, newReallocOptions("c5", 0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))

	assert.NoError(t, err)
	assert.Equal(t, 0, len(c5.ResourceMeta.CPU))

	err = c.ReallocResource(ctx, newReallocOptions("c6", 0.1, 2*int64(units.MiB), nil, types.TriTrue, types.TriKeep))

	assert.NoError(t, err)
	assert.NotEmpty(t, c6.CPURequest)

	node3.CPU = types.CPUMap{"0": 10, "1": 70, "2": 100, "3": 100}
	err = c.ReallocResource(ctx, newReallocOptions("c5", -0.1, 2*int64(units.MiB), nil, types.TriTrue, types.TriKeep))

	assert.NoError(t, err)
	assert.NotEmpty(t, c6.CPURequest)
	assert.NotEmpty(t, c5.CPURequest)
	err = c.ReallocResource(ctx, newReallocOptions("c6", -0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))

	assert.NoError(t, err)
	assert.Equal(t, 0, len(c5.CPURequest))
	assert.Equal(t, 0, len(c6.CPURequest))
}
*/
