package calcium

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	"time"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func TestPodResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	podname := "testpod"
	nodename := "testnode"
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.PodResource(ctx, podname)
	assert.Error(t, err)
	node := &types.Node{
		Name:           nodename,
		CPU:            types.CPUMap{"0": 0, "1": 10},
		MemCap:         2,
		InitCPU:        types.CPUMap{"0": 100, "1": 100},
		InitMemCap:     6,
		InitStorageCap: 10,
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by ListNodeContainers
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.PodResource(ctx, podname)
	assert.Error(t, err)
	containers := []*types.Container{
		{
			Memory: 1,
			CPU:    types.CPUMap{"0": 100, "1": 30},
			Quota:  1.3,
		},
		{
			Memory:  2,
			CPU:     types.CPUMap{"1": 50},
			Quota:   0.5,
			Storage: 1,
		},
	}
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// success
	r, err := c.PodResource(ctx, podname)
	assert.NoError(t, err)
	assert.Len(t, r.CPUPercents, 1)
	assert.Len(t, r.MemoryPercents, 1)
	assert.Len(t, r.StoragePercents, 1)
	assert.False(t, r.Verifications[nodename])
	assert.NotEmpty(t, r.Details[nodename])
}

func TestNodeResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	nodename := "testnode"
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	node := &types.Node{
		Name:           nodename,
		CPU:            types.CPUMap{"0": 0, "1": 10},
		MemCap:         2,
		InitCPU:        types.CPUMap{"0": 100, "1": 100},
		InitMemCap:     6,
		NUMAMemory:     types.NUMAMemory{"0": 1, "1": 1},
		InitNUMAMemory: types.NUMAMemory{"0": 3, "1": 3},
	}
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// failed by GetNode
	store.On("GetNode", ctx, nodename).Return(nil, types.ErrNoETCD).Once()
	_, err := c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, nodename).Return(node, nil)
	// failed by list node containers
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	containers := []*types.Container{
		{
			Memory: 1,
			CPU:    types.CPUMap{"0": 100, "1": 30},
			Quota:  1.3,
		},
		{
			Memory: 2,
			CPU:    types.CPUMap{"1": 50},
			Quota:  0.5,
		},
	}
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	store.On("UpdateNode", mock.Anything, mock.Anything).Return(nil)
	// success but container inspect failed
	nr, err := c.NodeResource(ctx, nodename, true)
	assert.NoError(t, err)
	assert.Equal(t, nr.Name, nodename)
	assert.NotEmpty(t, nr.Details)
	assert.False(t, nr.Verification)
	details := strings.Join(nr.Details, ",")
	assert.Contains(t, details, "inspect failed")
}

func TestAllocResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	podname := "testpod"
	opts := &types.DeployOptions{
		Podname: podname,
	}
	config := types.Config{
		LockTimeout: time.Duration(time.Second * 3),
	}
	c.config = config
	n1 := "n2"
	n2 := "n2"
	nodes := []*types.Node{
		{
			Name:      n1,
			Available: false,
			Labels:    map[string]string{"test": "1"},
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
		{
			Name:      n2,
			Available: true,
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)

	testAllocFailedAsGetNodesByPodError(t, c, opts)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)

	testAllocFailedAsCreateLockError(t, c, opts)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(&dummyLock{}, nil)

	testAllocFailedAsNoLabels(t, c, opts)

	// Defines for below.
	opts.Nodenames = []string{n2}
	testAllocFailedAsGetNodeError(t, c, opts)

	// Mocks for all of rest.
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nodes[1], nil)
	// define nodesInfo
	nodesInfo := []types.NodeInfo{
		{
			Name:     n2,
			CPUMap:   types.CPUMap{"0": 100},
			MemCap:   100,
			Count:    1,  // 1 exists
			Deploy:   3,  // deploy 1
			Capacity: 10, // can deploy 10
		},
	}
	nodeCPUPlans := map[string][]types.CPUMap{
		n2: {
			{"0": 10},
			{"0": 10},
			{"0": 10},
			{"0": 10},
			{"0": 10},
		},
	}
	nodeVolumePlans := map[string][]types.VolumePlan{
		n2: {
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/dir": 100}},
			{types.MustToVolumeBinding("AUTO:/data:rw:100"): types.VolumeMap{"/mount": 100}},
		},
	}

	testAllocFailedAsMakeDeployStatusError(t, c, opts)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nil)

	testAllocFailedAsInsufficientMemory(t, c, opts)

	sched := c.scheduler.(*schedulermocks.Scheduler)
	defer sched.AssertExpectations(t)

	total := 3
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, total, nil)

	testAllocFailedAsInsufficientStorage(t, c, opts)
	sched.On("SelectStorageNodes", mock.Anything, mock.Anything).Return(nodesInfo, total, nil)

	testAllocFailedAsInsufficientCPU(t, c, opts)
	sched.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nodeCPUPlans, total, nil)

	testAllocFailedAsInsufficientVolume(t, c, opts)
	sched.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nodesInfo, nodeVolumePlans, total, nil)

	testAllocFailedAsWrongDeployMethod(t, c, opts)

	testAllocFailedAsCommonDivisionError(t, c, opts)
	testAllocFailedAsGlobalDivisionError(t, c, opts)
	testAllocFailedAsEachDivisionError(t, c, opts)
	testAllocFailedAsFillDivisionError(t, c, opts)

	// Mocks for all.
	opts.DeployStrategy = strategy.Fill
	oldFillFunc := strategy.Plans[strategy.Fill]
	strategy.Plans[strategy.Fill] = func(nodesInfo []types.NodeInfo, need, _, limit int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
		return nodesInfo, nil
	}
	defer func() {
		strategy.Plans[strategy.Fill] = oldFillFunc
	}()

	testAllocFailedAsUpdateNodeResourceError(t, c, opts)
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)

	testAllocFailedAsSaveProcessingError(t, c, opts)
	store.On("SaveProcessing",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)

	// success
	opts.CPUBind = true
	nsi, err := c.doAllocResource(ctx, opts)
	assert.NoError(t, err)
	assert.Len(t, nsi, 1)
	assert.Equal(t, nsi[0].Name, n2)
	// stupid race condition
}

func testAllocFailedAsGetNodesByPodError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	store := c.store.(*storemocks.Store)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsCreateLockError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsNoLabels(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.NodeLabels
	defer func() {
		opts.NodeLabels = ori
	}()

	store := c.store.(*storemocks.Store)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	opts.NodeLabels = map[string]string{"test": "1"}
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsGetNodeError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	store := c.store.(*storemocks.Store)
	store.On("GetNode", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsMakeDeployStatusError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	store := c.store.(*storemocks.Store)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsInsufficientMemory(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	sched := c.scheduler.(*schedulermocks.Scheduler)
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrInsufficientMEM).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsInsufficientStorage(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	sched := c.scheduler.(*schedulermocks.Scheduler)
	sched.On("SelectStorageNodes", mock.Anything, mock.Anything).Return(nil, 0, types.ErrInsufficientStorage).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsInsufficientCPU(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.CPUBind
	defer func() {
		opts.CPUBind = ori
	}()

	opts.CPUBind = true
	opts.CPUQuota = -1
	sched := c.scheduler.(*schedulermocks.Scheduler)
	sched.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientCPU).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsWrongDeployMethod(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.DeployStrategy
	defer func() {
		opts.DeployStrategy = ori
	}()

	opts.DeployStrategy = "invalid"
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsCommonDivisionError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.DeployStrategy
	opts.DeployStrategy = strategy.Auto
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(nodesInfo []types.NodeInfo, need, total, _ int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
		return nil, types.ErrInsufficientRes
	}
	defer func() {
		opts.DeployStrategy = ori
		strategy.Plans[strategy.Auto] = old
	}()

	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsGlobalDivisionError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.DeployStrategy
	opts.DeployStrategy = strategy.Global
	old := strategy.Plans[strategy.Global]
	strategy.Plans[strategy.Global] = func(nodesInfo []types.NodeInfo, need, total, _ int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
		return nil, types.ErrInsufficientRes
	}
	defer func() {
		opts.DeployStrategy = ori
		strategy.Plans[strategy.Global] = old
	}()

	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsEachDivisionError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.DeployStrategy
	opts.DeployStrategy = strategy.Each
	old := strategy.Plans[strategy.Each]
	strategy.Plans[strategy.Each] = func(nodesInfo []types.NodeInfo, need, _, limit int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
		return nil, types.ErrInsufficientRes
	}
	defer func() {
		opts.DeployStrategy = ori
		strategy.Plans[strategy.Each] = old
	}()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsFillDivisionError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	ori := opts.DeployStrategy
	opts.DeployStrategy = strategy.Fill
	old := strategy.Plans[strategy.Fill]
	strategy.Plans[strategy.Fill] = func(nodesInfo []types.NodeInfo, need, _, limit int, resourceType types.ResourceType) ([]types.NodeInfo, error) {
		return nil, types.ErrInsufficientRes
	}
	defer func() {
		opts.DeployStrategy = ori
		strategy.Plans[strategy.Fill] = old
	}()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsUpdateNodeResourceError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	store := c.store.(*storemocks.Store)
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsSaveProcessingError(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	store := c.store.(*storemocks.Store)
	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}

func testAllocFailedAsInsufficientVolume(t *testing.T, c *Calcium, opts *types.DeployOptions) {
	sched := c.scheduler.(*schedulermocks.Scheduler)
	sched.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nil, nil, 0, types.ErrInsufficientVolume).Once()
	_, err := c.doAllocResource(context.Background(), opts)
	assert.Error(t, err)
}
