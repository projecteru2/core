package calcium

import (
	"context"
	"testing"

	"github.com/docker/go-units"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	complexscheduler "github.com/projecteru2/core/scheduler/complex"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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

func TestRealloc(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	c.config.Scheduler.ShareBase = 100

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:       "node1",
			MemCap:     int64(units.GiB),
			CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
			InitCPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Endpoint:   "http://1.1.1.1:1",
			NUMA:       types.NUMA{"2": "0"},
			NUMAMemory: types.NUMAMemory{"0": 100000},
			Volume:     types.VolumeMap{"/dir0": 100},
		},
		Engine: engine,
	}

	newC1 := func(context.Context, []string) []*types.Workload {
		return []*types.Workload{
			{
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
			},
		}
	}

	newC2 := func(context.Context, []string) []*types.Workload {
		return []*types.Workload{
			{
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
			},
		}
	}

	store.On("GetWorkloads", mock.Anything, []string{"c1"}).Return(newC1, nil)
	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	err := c.ReallocResource(ctx, newReallocOptions("c1", -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "ETCD must be set")
	store.AssertExpectations(t)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	// failed by newCPU < 0
	err = c.ReallocResource(ctx, newReallocOptions("c1", -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "limit or request less than 0: bad `CPU` value")
	store.AssertExpectations(t)

	// failed by GetNode
	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrNoETCD).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "ETCD must be set")
	store.AssertExpectations(t)

	// failed by no enough mem
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	simpleMockScheduler := &schedulermocks.Scheduler{}
	scheduler.InitSchedulerV1(simpleMockScheduler)
	c.scheduler = simpleMockScheduler
	simpleMockScheduler.On("ReselectCPUNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.ScheduleInfo{}, nil, 0, types.ErrInsufficientMEM).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "cannot alloc a plan, not enough memory")
	store.AssertExpectations(t)
	simpleMockScheduler.AssertExpectations(t)

	// failed by wrong total
	simpleMockScheduler.On("ReselectCPUNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.ScheduleInfo{}, nil, 0, nil).Once()
	simpleMockScheduler.On("SelectStorageNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 100, nil)
	nodeVolumePlans := map[string][]types.VolumePlan{
		"node1": {{types.MustToVolumeBinding("AUTO:/data:rw:50"): types.VolumeMap{"/dir0": 50}}},
	}
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"})).Return(nil, nodeVolumePlans, 1, nil).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "not enough resource")
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// vaild cpu plans
	nodeCPUPlans := map[string][]types.CPUMap{
		node1.Name: {
			{"3": 100},
			{"2": 100},
		},
	}
	simpleMockScheduler.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nil, 100, nil).Once()
	// failed by apply resource
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrBadWorkloadID).Once()
	// reset node
	node1 = &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "node1",
			MemCap:   int64(units.GiB),
			CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
			Endpoint: "http://1.1.1.1:1",
		},
		Engine: engine,
	}
	store.On("GetWorkloads", mock.Anything, []string{"c2"}).Return(newC2, nil)
	err = c.ReallocResource(ctx, newReallocOptions("c2", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "workload ID must be length of 64")
	assert.Equal(t, node1.CPU["2"], int64(10))
	assert.Equal(t, node1.MemCap, int64(units.GiB))
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// failed by update workload
	simpleMockScheduler.On("ReselectCPUNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.ScheduleInfo{}, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nil, 100, nil).Once()
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(types.ErrBadWorkloadID).Times(1)
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "workload ID must be length of 64")
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
	simpleMockScheduler.On("ReselectCPUNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.ScheduleInfo{}, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nodeVolumePlans, 4, nil).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"}), types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "incompatible volume plans: cannot alloc a plan, not enough volume")
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// failed by volume schedule error
	simpleMockScheduler.On("ReselectCPUNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.ScheduleInfo{}, nodeCPUPlans, 2, nil).Once()
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientVolume).Once()
	err = c.ReallocResource(ctx, newReallocOptions("c1", 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:1"}), types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "cannot alloc a plan, not enough volume")
	simpleMockScheduler.AssertExpectations(t)
	store.AssertExpectations(t)

	// good to go
	// rest everything
	node2 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:       "node2",
			MemCap:     int64(units.GiB),
			CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
			InitCPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Endpoint:   "http://1.1.1.1:1",
			NUMA:       types.NUMA{"2": "0"},
			NUMAMemory: types.NUMAMemory{"0": 100000},
			Volume:     types.VolumeMap{"/dir0": 200, "/dir1": 200, "/dir2": 200},
		},
		VolumeUsed: int64(300),
		Engine:     engine,
	}
	c3 := &types.Workload{
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
	simpleMockScheduler.On("ReselectCPUNodes", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(resourcetypes.ScheduleInfo{}, nodeCPUPlans, 2, nil)
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nodeVolumePlans, 2, nil)
	store.On("GetNode", mock.Anything, "node2").Return(node2, nil)
	store.On("GetWorkloads", mock.Anything, []string{"c3"}).Return([]*types.Workload{c3}, nil)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(types.ErrBadWorkloadID).Times(1)
	err = c.ReallocResource(ctx, newReallocOptions("c3", 0.1, 2*int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data0:rw:-50"}), types.TriKeep, types.TriKeep))
	assert.EqualError(t, err, "workload ID must be length of 64")
	assert.Equal(t, node2.CPU["3"], int64(100))
	assert.Equal(t, node2.CPU["2"], int64(100))
	assert.Equal(t, node2.MemCap, int64(units.GiB)+5*int64(units.MiB))
	assert.Equal(t, node2.Volume, types.VolumeMap{"/dir0": 300, "/dir1": 400, "/dir2": 200})
	assert.Equal(t, node2.VolumeUsed, int64(0))
	store.AssertExpectations(t)
	simpleMockScheduler.AssertExpectations(t)
}

func TestReallocBindCpu(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	store := &storemocks.Store{}
	c.store = store
	pod1 := &types.Pod{
		Name: "p1",
	}
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
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
		NodeMeta: types.NodeMeta{
			Name:       "node3",
			MemCap:     int64(units.GiB),
			CPU:        types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
			InitCPU:    types.CPUMap{"0": 100, "1": 100, "2": 100, "3": 100},
			Endpoint:   "http://1.1.1.1:1",
			NUMA:       types.NUMA{"2": "0"},
			NUMAMemory: types.NUMAMemory{"0": 100000},
			Volume:     types.VolumeMap{"/dir0": 200, "/dir1": 200, "/dir2": 200},
		},
		VolumeUsed: int64(300),
		CPUUsed:    2.1,
		Engine:     engine,
	}
	c5 := &types.Workload{
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
	c6 := &types.Workload{
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
	store.On("GetWorkloads", mock.Anything, []string{"c5"}).Return([]*types.Workload{c5}, nil)
	store.On("GetWorkloads", mock.Anything, []string{"c6"}).Return([]*types.Workload{c6}, nil)
	store.On("GetWorkloads", mock.Anything, []string{"c6", "c5"}).Return([]*types.Workload{c5, c6}, nil)
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil)

	// failed by UpdateNodes
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(types.ErrBadWorkloadID).Once()
	err := c.ReallocResource(ctx, newReallocOptions("c5", 0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))
	assert.Error(t, err)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)

	err = c.ReallocResource(ctx, newReallocOptions("c5", 0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))
	assert.NoError(t, err)
	assert.Equal(t, 0, len(c5.ResourceMeta.CPU))

	err = c.ReallocResource(ctx, newReallocOptions("c6", 0.1, 2*int64(units.MiB), nil, types.TriTrue, types.TriKeep))
	assert.NoError(t, err)
	assert.NotEmpty(t, c6.ResourceMeta.CPU)

	node3.CPU = types.CPUMap{"0": 10, "1": 70, "2": 100, "3": 100}
	err = c.ReallocResource(ctx, newReallocOptions("c5", -0.1, 2*int64(units.MiB), nil, types.TriTrue, types.TriKeep))

	assert.NoError(t, err)
	assert.NotEmpty(t, c5.ResourceMeta.CPU)
	err = c.ReallocResource(ctx, newReallocOptions("c6", -0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))

	assert.NoError(t, err)
	assert.Equal(t, 0, len(c6.ResourceMeta.CPU))
}
