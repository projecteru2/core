package calcium

import (
	"context"
	"testing"

	complexscheduler "github.com/projecteru2/core/scheduler/complex"

	"github.com/stretchr/testify/assert"

	"github.com/docker/go-units"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/mock"
)

func newReallocOptions(ids []string, cpu float64, memory int64, vbs types.VolumeBindings, bindCPU, memoryLimit types.TriOptions) *types.ReallocOptions {
	return &types.ReallocOptions{
		IDs:         ids,
		CPU:         cpu,
		Memory:      memory,
		Volumes:     vbs,
		BindCPU:     bindCPU,
		MemoryLimit: memoryLimit,
	}
}

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
		ID:         "c1",
		Podname:    "p1",
		Engine:     engine,
		Memory:     5 * int64(units.MiB),
		Quota:      0.9,
		CPU:        types.CPUMap{"2": 90},
		Nodename:   "node1",
		VolumePlan: types.VolumePlan{types.MustToVolumeBinding("AUTO:/data:rw:50"): types.VolumeMap{"/dir0": 50}},
		Volumes:    types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"}),
	}

	c2 := &types.Container{
		ID:       "c2",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		Nodename: "node1",
	}

	store.On("GetContainers", mock.Anything, []string{"c1"}).Return([]*types.Container{c1}, nil)
	store.On("GetContainers", mock.Anything, []string{"c2"}).Return([]*types.Container{c2}, nil)
	store.On("GetContainers", mock.Anything, []string{"c1", "c2"}).Return([]*types.Container{c1, c2}, nil)
	// failed by lock
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	ch, err := c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by GetPod
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("GetPod", mock.Anything, mock.Anything).Return(pod1, nil)
	// failed by newCPU < 0
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, -1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by GetNode
	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrNoETCD).Once()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, 0.1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)
	// failed by memory not enough
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, 0.1, 2*int64(units.GiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by no new CPU Plan
	simpleMockScheduler := &schedulermocks.Scheduler{}
	c.scheduler = simpleMockScheduler
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientMEM).Once()
	nodeVolumePlans := map[string][]types.VolumePlan{
		"c1": {{types.MustToVolumeBinding("AUTO:/data:rw:50"): types.VolumeMap{"/dir0": 50}}},
	}
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"})).Return(nil, nodeVolumePlans, 1, nil)
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by wrong total
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, nil).Once()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// vaild cpu plans
	nodeCPUPlans := map[string][]types.CPUMap{
		node1.Name: {
			{"3": 100},
			{"2": 100},
		},
	}
	simpleMockScheduler.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nodeCPUPlans, 2, nil).Times(5)
	// failed by apply resource
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Twice()
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil).Twice()
	// update node failed
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	// reset node
	node1 = &types.Node{
		Name:     "node1",
		MemCap:   int64(units.GiB),
		CPU:      types.CPUMap{"0": 10, "1": 70, "2": 10, "3": 100},
		Engine:   engine,
		Endpoint: "http://1.1.1.1:1",
	}
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1", "c2"}, 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// check node resource as usual
	assert.Equal(t, node1.CPU["2"], int64(10))
	assert.Equal(t, node1.MemCap, int64(units.GiB))
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	// failed by update container
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Once()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1", "c2"}, 0.1, 2*int64(units.MiB), nil, types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by volume binding incompatible
	nodeVolumePlans = map[string][]types.VolumePlan{
		node1.Name: {
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir1": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir2": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir3": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir4": 100}},
		},
	}
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data:rw:100"})).Return(nil, nodeVolumePlans, 4, nil).Once()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:50"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed by volume schedule error
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientVolume).Once()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1"}, 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:1"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	// failed due to re-volume plan less then container number
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nodeVolumePlans, 0, nil).Twice()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c1", "c2"}, 0.1, int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data:rw:1"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
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
		Memory:  5 * int64(units.MiB),
		Quota:   0.9,
		CPU:     types.CPUMap{"2": 90},
		Volumes: types.MustToVolumeBindings([]string{"AUTO:/data0:rw:100", "AUTO:/data1:rw:200"}),
		VolumePlan: types.VolumePlan{
			types.MustToVolumeBinding("AUTO:/data0:rw:100"): types.VolumeMap{"/dir0": 100},
			types.MustToVolumeBinding("AUTO:/data1:rw:200"): types.VolumeMap{"/dir1": 200},
		},
		Nodename: "node2",
	}
	c4 := &types.Container{
		ID:       "c4",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		Volumes:  types.MustToVolumeBindings([]string{"/tmp:/tmp", "/var/log:/var/log:rw:300"}),
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
	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, types.MustToVolumeBindings([]string{"AUTO:/data0:rw:50", "AUTO:/data1:rw:200"})).Return(nil, nodeVolumePlans, 2, nil)
	store.On("GetNode", mock.Anything, "node2").Return(node2, nil)
	store.On("GetContainers", mock.Anything, []string{"c3", "c4"}).Return([]*types.Container{c3, c4}, nil)
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(types.ErrBadContainerID).Twice()
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c3", "c4"}, 0.1, 2*int64(units.MiB), types.MustToVolumeBindings([]string{"AUTO:/data0:rw:-50"}), types.TriKeep, types.TriKeep))
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	assert.Equal(t, node2.CPU["3"], int64(0))
	assert.Equal(t, node2.CPU["2"], int64(100))
	assert.Equal(t, node2.MemCap, int64(units.GiB)-4*int64(units.MiB))
	assert.Equal(t, node2.Volume, types.VolumeMap{"/dir0": 250, "/dir1": 200, "/dir2": 200})
	assert.Equal(t, node2.VolumeUsed, int64(250))

}

func TestReallocVolume(t *testing.T) {
	c := NewTestCluster()
	store := &storemocks.Store{}
	c.store = store

	simpleMockScheduler := &schedulermocks.Scheduler{}
	c.scheduler = simpleMockScheduler
	engine := &enginemocks.API{}

	node1 := &types.Node{
		Name: "node1",
	}

	c1 := &types.Container{
		ID:       "c1",
		Engine:   engine,
		Nodename: "node1",
		VolumePlan: types.VolumePlan{
			types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
			types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir0": 100},
			types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir0": 0},
			types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
		},
	}

	newVbs := types.MustToVolumeBindings([]string{
		"AUTO:/data:rw:0",
		"AUTO:/data1:rw:0",
		"AUTO:/data2:ro:110",
		"AUTO:/data3:rw:20",
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

	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, newVbs).Return(nil, newPlans, 1, nil).Once()
	_, err := c.reallocVolume(node1, []*types.Container{c1}, newVbs)
	assert.Error(t, err, "not compatible")

	// test 2: modify unlimited volume map for compatible requirement

	newPlans = map[string][]types.VolumePlan{
		"node1": {
			{
				*newVbs[0]: types.VolumeMap{"/dir1": 0},
				*newVbs[1]: types.VolumeMap{"/dir1": 0},
				*newVbs[2]: types.VolumeMap{"/dir1": 110},
				*newVbs[3]: types.VolumeMap{"/dir1": 20},
			},
			{
				*newVbs[0]: types.VolumeMap{"/dir1": 0},
				*newVbs[1]: types.VolumeMap{"/dir1": 0},
				*newVbs[2]: types.VolumeMap{"/dir0": 110},
				*newVbs[3]: types.VolumeMap{"/dir0": 20},
			},
		},
	}

	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, newVbs).Return(nil, newPlans, 2, nil).Once()

	plans, err := c.reallocVolume(node1, []*types.Container{c1}, newVbs)
	assert.Nil(t, err)
	assert.Equal(t, plans[c1][*newVbs[0]], types.VolumeMap{"/dir0": 0})
	assert.Equal(t, plans[c1][*newVbs[1]], types.VolumeMap{"/dir0": 0})
	assert.Equal(t, plans[c1][*newVbs[2]], types.VolumeMap{"/dir0": 110})
	assert.Equal(t, plans[c1][*newVbs[3]], types.VolumeMap{"/dir0": 20})

	// test 3: multiple containers search compatible respective plans

	newPlans = map[string][]types.VolumePlan{
		"node1": {
			{
				*newVbs[0]: types.VolumeMap{"/dir1": 0},
				*newVbs[1]: types.VolumeMap{"/dir1": 0},
				*newVbs[2]: types.VolumeMap{"/dir0": 110},
				*newVbs[3]: types.VolumeMap{"/dir1": 20},
			},
			{
				*newVbs[0]: types.VolumeMap{"/dir1": 0},
				*newVbs[1]: types.VolumeMap{"/dir1": 0},
				*newVbs[2]: types.VolumeMap{"/dir0": 110},
				*newVbs[3]: types.VolumeMap{"/dir0": 20},
			},
			{
				*newVbs[0]: types.VolumeMap{"/dir1": 0},
				*newVbs[1]: types.VolumeMap{"/dir1": 0},
				*newVbs[2]: types.VolumeMap{"/dir1": 110},
				*newVbs[3]: types.VolumeMap{"/dir0": 20},
			},
		},
	}

	c2 := &types.Container{
		ID:       "c2",
		Engine:   engine,
		Nodename: "node1",
		VolumePlan: types.VolumePlan{
			types.MustToVolumeBinding("AUTO:/data:rw:0"):    types.VolumeMap{"/dir0": 0},
			types.MustToVolumeBinding("AUTO:/data1:rw:100"): types.VolumeMap{"/dir1": 100},
			types.MustToVolumeBinding("AUTO:/data2:rw:0"):   types.VolumeMap{"/dir1": 0},
			types.MustToVolumeBinding("AUTO:/data3:rw:600"): types.VolumeMap{"/dir0": 600},
		},
	}

	simpleMockScheduler.On("SelectVolumeNodes", mock.Anything, newVbs).Return(nil, newPlans, 3, nil).Once()

	plans, err = c.reallocVolume(node1, []*types.Container{c1, c2}, newVbs)
	assert.Nil(t, err)
	assert.Equal(t, plans[c1][*newVbs[0]], types.VolumeMap{"/dir0": 0})
	assert.Equal(t, plans[c1][*newVbs[1]], types.VolumeMap{"/dir0": 0})
	assert.Equal(t, plans[c1][*newVbs[2]], types.VolumeMap{"/dir0": 110})
	assert.Equal(t, plans[c1][*newVbs[3]], types.VolumeMap{"/dir0": 20})
	assert.Equal(t, plans[c2][*newVbs[0]], types.VolumeMap{"/dir0": 0})
	assert.Equal(t, plans[c2][*newVbs[1]], types.VolumeMap{"/dir1": 0})
	assert.Equal(t, plans[c2][*newVbs[2]], types.VolumeMap{"/dir1": 110})
	assert.Equal(t, plans[c2][*newVbs[3]], types.VolumeMap{"/dir0": 20})
}

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
		ID:       "c5",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		CPU:      types.CPUMap{"2": 90},
		Nodename: "node3",
	}
	c6 := &types.Container{
		ID:       "c6",
		Podname:  "p1",
		Engine:   engine,
		Memory:   5 * int64(units.MiB),
		Quota:    0.9,
		Nodename: "node3",
	}

	store.On("GetNode", mock.Anything, "node3").Return(node3, nil)
	store.On("GetContainers", mock.Anything, []string{"c5"}).Return([]*types.Container{c5}, nil)
	store.On("GetContainers", mock.Anything, []string{"c6"}).Return([]*types.Container{c6}, nil)
	store.On("GetContainers", mock.Anything, []string{"c6", "c5"}).Return([]*types.Container{c5, c6}, nil)
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateContainer", mock.Anything, mock.Anything).Return(nil)
	ch, err := c.ReallocResource(ctx, newReallocOptions([]string{"c5"}, 0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))
	for r := range ch {
		assert.NoError(t, r.Error)
	}
	assert.NoError(t, err)
	assert.Empty(t, c5.CPU)

	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c6"}, 0.1, 2*int64(units.MiB), nil, types.TriTrue, types.TriKeep))
	for r := range ch {
		assert.NoError(t, r.Error)
	}
	assert.NoError(t, err)
	assert.NotEmpty(t, c6.CPU)

	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c6", "c5"}, -0.1, 2*int64(units.MiB), nil, types.TriTrue, types.TriKeep))
	for r := range ch {
		assert.NoError(t, r.Error)
	}
	assert.NoError(t, err)
	assert.NotEmpty(t, c6.CPU)
	assert.NotEmpty(t, c5.CPU)
	ch, err = c.ReallocResource(ctx, newReallocOptions([]string{"c6", "c5"}, -0.1, 2*int64(units.MiB), nil, types.TriFalse, types.TriKeep))
	for r := range ch {
		assert.NoError(t, r.Error)
	}
	assert.NoError(t, err)
	assert.Empty(t, c6.CPU)
	assert.Empty(t, c5.CPU)

}
