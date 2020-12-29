package calcium

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
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
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.PodResource(ctx, podname)
	assert.Error(t, err)
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:           nodename,
			CPU:            types.CPUMap{"0": 0, "1": 10},
			MemCap:         2,
			InitCPU:        types.CPUMap{"0": 100, "1": 100},
			InitMemCap:     6,
			InitStorageCap: 10,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by ListNodeWorkloads
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.PodResource(ctx, podname)
	assert.Error(t, err)
	workloads := []*types.Workload{
		{
			ResourceMeta: types.ResourceMeta{
				MemoryRequest:   1,
				MemoryLimit:     1,
				CPU:             types.CPUMap{"0": 100, "1": 30},
				CPUQuotaRequest: 1.3,
				CPUQuotaLimit:   1.3,
			},
		},
		{
			ResourceMeta: types.ResourceMeta{
				MemoryLimit:     2,
				MemoryRequest:   2,
				CPU:             types.CPUMap{"1": 50},
				CPUQuotaRequest: 0.5,
				CPUQuotaLimit:   0.5,
				StorageRequest:  1,
				StorageLimit:    1,
			},
		},
	}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// success
	r, err := c.PodResource(ctx, podname)
	assert.NoError(t, err)
	assert.Equal(t, r.NodesResource[0].CPUPercent, 0.9)
	assert.Equal(t, r.NodesResource[0].MemoryPercent, 0.5)
	assert.Equal(t, r.NodesResource[0].StoragePercent, 0.1)
	assert.NotEmpty(t, r.NodesResource[0].Diffs)
}

func TestNodeResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	nodename := "testnode"
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(context.TODO(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:           nodename,
			CPU:            types.CPUMap{"0": 0, "1": 10},
			MemCap:         2,
			InitCPU:        types.CPUMap{"0": 100, "1": 100},
			InitMemCap:     6,
			NUMAMemory:     types.NUMAMemory{"0": 1, "1": 1},
			InitNUMAMemory: types.NUMAMemory{"0": 3, "1": 3},
		},
	}
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// fail by validating
	_, err := c.NodeResource(ctx, "", false)
	assert.Error(t, err)
	// failed by GetNode
	store.On("GetNode", ctx, nodename).Return(nil, types.ErrNoETCD).Once()
	_, err = c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, nodename).Return(node, nil)
	// failed by list node workloads
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	workloads := []*types.Workload{
		{
			ResourceMeta: types.ResourceMeta{
				MemoryRequest:   1,
				MemoryLimit:     1,
				CPU:             types.CPUMap{"0": 100, "1": 30},
				CPUQuotaRequest: 1.3,
				CPUQuotaLimit:   1.3,
			},
		},
		{
			ResourceMeta: types.ResourceMeta{
				MemoryRequest:   2,
				MemoryLimit:     2,
				CPU:             types.CPUMap{"1": 50},
				CPUQuotaRequest: 0.5,
				CPUQuotaLimit:   0.5,
			},
		},
	}
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(workloads, nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	// success but workload inspect failed
	nr, err := c.NodeResource(ctx, nodename, true)
	assert.NoError(t, err)
	assert.Equal(t, nr.Name, nodename)
	assert.NotEmpty(t, nr.Diffs)
	details := strings.Join(nr.Diffs, ",")
	assert.Contains(t, details, "inspect failed")
}

func TestAllocResource(t *testing.T) {
	c := NewTestCluster()
	scheduler.InitSchedulerV1(c.scheduler)
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
	nodeMap := map[string]*types.Node{
		n1: {
			NodeMeta: types.NodeMeta{
				Name:   n1,
				Labels: map[string]string{"test": "1"},
				CPU:    types.CPUMap{"0": 100},
				MemCap: 100,
			},
			Available: false,
		},
		n2: {
			NodeMeta: types.NodeMeta{
				Name:   n2,
				CPU:    types.CPUMap{"0": 100},
				MemCap: 100,
			},
			Available: true,
		},
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)

	// Defines for below.
	opts.Nodenames = []string{n2}

	// define scheduleInfos
	scheduleInfos := []resourcetypes.ScheduleInfo{
		{
			NodeMeta: types.NodeMeta{
				Name:   n2,
				CPU:    types.CPUMap{"0": 100},
				MemCap: 100,
			},
			Capacity: 10, // can deploy 10
		},
	}

	sched := c.scheduler.(*schedulermocks.Scheduler)
	defer sched.AssertExpectations(t)
	total := 3
	sched.On("SelectStorageNodes", mock.Anything, mock.Anything).Return(scheduleInfos, total, nil)
	sched.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(scheduleInfos, nil, total, nil)
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrInsufficientMEM).Once()
	testAllocFailedAsInsufficientMemory(t, c, opts, nodeMap)

	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(scheduleInfos, total, nil)
	testAllocFailedAsMakeDeployStatusError(t, c, opts, nodeMap)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	testAllocFailedAsWrongDeployMethod(t, c, opts, nodeMap)
	testAllocFailedAsCommonDivisionError(t, c, opts, nodeMap)

	// Mocks for all.
	opts.DeployStrategy = strategy.Fill
	oldFillFunc := strategy.Plans[strategy.Fill]
	strategy.Plans[strategy.Fill] = func(sis []strategy.Info, need, _, limit int, resourceType types.ResourceType) (map[string]int, error) {
		dis := make(map[string]int)
		for _, si := range sis {
			dis[si.Nodename] = 3
		}
		return dis, nil
	}
	defer func() {
		strategy.Plans[strategy.Fill] = oldFillFunc
	}()

	// success
	opts.ResourceOpts = types.ResourceOptions{CPUQuotaLimit: 1, MemoryLimit: 1, StorageLimit: 1}
	_, _, err := c.doAllocResource(ctx, nodeMap, opts)
	assert.NoError(t, err)
}

func testAllocFailedAsMakeDeployStatusError(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {
	store := c.store.(*storemocks.Store)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
}

func testAllocFailedAsInsufficientMemory(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {
	opts.ResourceOpts = types.ResourceOptions{CPUQuotaLimit: 1, MemoryLimit: 1}
	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
}

func testAllocFailedAsWrongDeployMethod(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {

	opts.DeployStrategy = "invalid"
	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
	opts.DeployStrategy = "AUTO"
}

func testAllocFailedAsCommonDivisionError(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {
	opts.DeployStrategy = strategy.Auto
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(_ []strategy.Info, need, total, _ int, resourceType types.ResourceType) (map[string]int, error) {
		return nil, types.ErrInsufficientRes
	}
	defer func() {
		strategy.Plans[strategy.Auto] = old
	}()

	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
}
